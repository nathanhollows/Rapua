// Package services provides entity deletion with transaction safety.
// Relies on ON DELETE CASCADE constraints at the database level to handle
// most child record cleanup. Application code explicitly deletes blocks
// (whose owner_id is polymorphic and cannot carry a FK constraint) and
// handles upload file cleanup.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// errObjectiveAlreadyGone means the row a delete named is not there. Internal:
// callers see an idempotent no-op, not an error.
var errObjectiveAlreadyGone = errors.New("objective already deleted")

// ErrCannotDeleteRoot is returned when a delete names a quest's root objective.
// The root goes only with the quest it belongs to.
var ErrCannotDeleteRoot = errors.New("the root objective cannot be deleted")

// DeleteService coordinates deletions. Database-level ON DELETE CASCADE
// handles child record cleanup; this service handles auth, file cleanup,
// and denormalized data updates.
type DeleteService struct {
	transactor    db.Transactor
	instanceRepo  repositories.QuestRepository
	teamRepo      repositories.RunRepository
	uploadsRepo   repositories.UploadsRepository
	objectiveRepo repositories.ObjectiveRepository
	db            *bun.DB
	uploadsDir    string
	logger        *slog.Logger
}

// NewDeleteService creates a new DeleteService with the provided dependencies.
func NewDeleteService(
	transactor db.Transactor,
	instanceRepo repositories.QuestRepository,
	teamRepo repositories.RunRepository,
	uploadsRepo repositories.UploadsRepository,
	objectiveRepo repositories.ObjectiveRepository,
	db *bun.DB,
	uploadsDir string,
	logger *slog.Logger,
) *DeleteService {
	return &DeleteService{
		transactor:    transactor,
		instanceRepo:  instanceRepo,
		teamRepo:      teamRepo,
		uploadsRepo:   uploadsRepo,
		objectiveRepo: objectiveRepo,
		db:            db,
		uploadsDir:    uploadsDir,
		logger:        logger,
	}
}

// DeleteUser deletes a user and all associated data.
// Cascade handles: instances, objectives, teams, states, settings, credits,
// purchases, start logs, notifications, facilitator tokens.
// Blocks are deleted explicitly because owner_id is polymorphic.
func (s *DeleteService) DeleteUser(ctx context.Context, userID string) error {
	// Collect upload file paths before the transaction deletes the rows.
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding user instances: %w", err)
	}
	var uploads []*models.Upload
	for _, inst := range instances {
		iu, uploadErr := s.uploadsRepo.SearchByCriteria(ctx, map[string]string{
			"quest_id": inst.ID,
		})
		if uploadErr != nil {
			return fmt.Errorf("fetching uploads for instance %s: %w", inst.ID, uploadErr)
		}
		uploads = append(uploads, iu...)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// blocks.owner_id has no FK; delete blocks for all objectives owned by
	// this user's quests, then delete the quest-level (start/finish) blocks.
	_, err = tx.NewDelete().Model((*models.Block)(nil)).
		Where("owner_id IN (SELECT o.id FROM objectives o JOIN quests q ON o.quest_id = q.id WHERE q.user_id = ?)", userID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting objective blocks for user: %w", err)
	}
	_, err = tx.NewDelete().Model((*models.Block)(nil)).
		Where("owner_id IN (SELECT id FROM quests WHERE user_id = ?)", userID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting instance blocks for user: %w", err)
	}

	// Single DELETE — cascade handles all other children
	_, err = tx.NewDelete().
		Model((*models.User)(nil)).
		Where("id = ?", userID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	if len(uploads) > 0 {
		go s.cleanupUploadFiles(context.Background(), uploads)
	}

	return nil
}

// DeleteBlock deletes a block. Cascade handles states.
func (s *DeleteService) DeleteBlock(ctx context.Context, blockID string) error {
	// Collect upload file paths before delete
	uploads, err := s.uploadsRepo.GetByBlockID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("fetching uploads: %w", err)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// Delete upload records within the transaction (avoids orphaned rows)
	if len(uploads) > 0 {
		ids := make([]string, len(uploads))
		for i, u := range uploads {
			ids[i] = u.ID
		}
		_, err = tx.NewDelete().
			Model((*models.Upload)(nil)).
			Where("id IN (?)", bun.In(ids)).
			Exec(ctx)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("deleting upload records: %w", err)
		}
	}

	// Delete block — cascade handles block states
	_, err = tx.NewDelete().
		Model((*models.Block)(nil)).
		Where("id = ?", blockID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting block: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	if len(uploads) > 0 {
		go s.cleanupUploadFiles(context.Background(), uploads)
	}

	return nil
}

// DeleteQuest deletes a quest and all its content.
// Returns ErrUserNotAuthenticated if userID doesn't own the instance.
// Blocks are deleted explicitly because owner_id is polymorphic.
func (s *DeleteService) DeleteQuest(ctx context.Context, userID, questID string) error {
	if userID == "" {
		return ErrUserNotAuthenticated
	}
	if questID == "" {
		return errors.New("questID cannot be empty")
	}

	// Auth check
	instance, err := s.instanceRepo.GetByID(ctx, questID)
	if err != nil {
		return fmt.Errorf("finding instance: %w", err)
	}
	if userID != instance.UserID {
		return ErrUserNotAuthenticated
	}

	// Collect upload file paths before the transaction deletes the rows.
	uploads, err := s.uploadsRepo.SearchByCriteria(ctx, map[string]string{
		"quest_id": questID,
	})
	if err != nil {
		return fmt.Errorf("fetching uploads for instance %s: %w", questID, err)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// blocks.owner_id has no FK so they won't be cascade-deleted automatically.
	_, err = tx.NewDelete().Model((*models.Block)(nil)).
		Where("owner_id IN (SELECT id FROM objectives WHERE quest_id = ?)", questID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting objective blocks: %w", err)
	}
	_, err = tx.NewDelete().Model((*models.Block)(nil)).
		Where("owner_id = ?", questID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting instance blocks: %w", err)
	}

	// Delete instance — cascade handles everything else
	_, err = tx.NewDelete().
		Model((*models.Quest)(nil)).
		Where("id = ?", questID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting instance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	if len(uploads) > 0 {
		go s.cleanupUploadFiles(context.Background(), uploads)
	}

	return nil
}

// DeleteObjective deletes blocks explicitly because blocks.owner_id has no FK
// to cascade from.
//
// The quest's root is refused. Its children would be adopted by the parent it
// does not have, leaving a quest with several parentless rows, which every walk
// from the root reads as an ambiguous tree and no screen offers a way to mend.
// Deleting the quest removes the root along with everything else.
func (s *DeleteService) DeleteObjective(ctx context.Context, objectiveID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	isRoot, err := s.objectiveIsRoot(ctx, tx, objectiveID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if isRoot {
		_ = tx.Rollback()
		return fmt.Errorf("%w: %s", ErrCannotDeleteRoot, objectiveID)
	}

	// Read the row before it goes: the children are placed under its parent
	// afterwards, and they inherit its draft state, and by then there is
	// nothing left to read either from.
	target, err := s.objectiveBeforeDelete(ctx, tx, objectiveID)
	if errors.Is(err, errObjectiveAlreadyGone) {
		// Which a double-submitted delete reaches. Nothing to delete and
		// nothing to reparent: carrying on would run the adoption with an empty
		// parent, and that is a real value matching every root the backfill
		// wrote, across every quest.
		return tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// Delete blocks first: cascade handles run_block_states.
	_, err = tx.NewDelete().
		Model((*models.Block)(nil)).
		Where("owner_id = ?", objectiveID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting blocks for objective: %w", err)
	}

	_, err = tx.NewDelete().
		Model((*models.Objective)(nil)).
		Where("id = ?", objectiveID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting objective: %w", err)
	}

	// After the delete, so the slot the row held is free for the placement to
	// settle into. parent_id has no FK, so nothing reparents the children on
	// their own, and left alone they are unreachable from the root.
	if err := s.adoptChildrenOfDeleted(ctx, tx, target); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// objectiveBeforeDelete returns the row about to be deleted, or
// errObjectiveAlreadyGone when it is not there.
//
// A sentinel rather than a zero value because every field the caller needs (the
// quest it belongs to, where it sat, whether it was parked) is a real value at
// zero, and acting on those would be acting on the wrong rows: an empty parent
// is the spelling migrated roots carry.
func (s *DeleteService) objectiveBeforeDelete(
	ctx context.Context, tx *bun.Tx, objectiveID string,
) (*models.Objective, error) {
	var objective models.Objective
	err := tx.NewSelect().
		Model(&objective).
		Column("id", "quest_id", "parent_id", "draft").
		Where("id = ?", objectiveID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errObjectiveAlreadyGone
		}
		return nil, fmt.Errorf("loading objective before delete: %w", err)
	}
	return &objective, nil
}

// objectiveIsRoot reports whether an objective is the top of its quest's tree.
// A row that is not there at all is not the root, so a repeated delete still
// reaches the idempotent path below rather than being refused.
func (s *DeleteService) objectiveIsRoot(ctx context.Context, tx *bun.Tx, objectiveID string) (bool, error) {
	var objective models.Objective
	err := tx.NewSelect().
		Model(&objective).
		Column("id", "parent_id").
		Where("id = ?", objectiveID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("loading objective before delete: %w", err)
	}
	return objective.ParentID == "", nil
}

// adoptChildrenOfDeleted moves an objective's children up to its own parent, so
// deleting a section removes the section rather than everything under it.
//
// The adopted children follow the destination's existing ones, and the whole
// list is renumbered densely. Carrying their old positions across would land
// them on positions already in use, and before that was a constraint it left
// the order to a tie broken by id, which is to say by nothing.
func (s *DeleteService) adoptChildrenOfDeleted(
	ctx context.Context,
	tx *bun.Tx,
	target *models.Objective,
) error {
	// Every read here is scoped to the quest. parent_id carries no foreign key
	// and an empty parent is a value the backfill actually wrote, so an
	// unscoped match on it reaches other quests' roots.
	var remaining []models.Objective
	if err := tx.NewSelect().
		Model(&remaining).
		Column("id", "position").
		Where("quest_id = ?", target.QuestID).
		Where("parent_id = ?", sql.NullString{String: target.ParentID, Valid: target.ParentID != ""}).
		Order("position ASC", "id ASC").
		Scan(ctx); err != nil {
		return fmt.Errorf("loading siblings of %s: %w", target.ID, err)
	}

	var adopted []models.Objective
	if err := tx.NewSelect().
		Model(&adopted).
		Column("id", "position").
		Where("quest_id = ?", target.QuestID).
		Where("parent_id = ?", target.ID).
		Order("position ASC", "id ASC").
		Scan(ctx); err != nil {
		return fmt.Errorf("loading children of %s: %w", target.ID, err)
	}

	// A drafted section was the only thing holding its children out of play, so
	// they carry the flag out with them. Left published they would rejoin a live
	// quest the moment the section they were hidden behind was tidied away.
	if target.Draft && len(adopted) > 0 {
		adoptedIDs := make([]string, len(adopted))
		for i, obj := range adopted {
			adoptedIDs[i] = obj.ID
		}
		if _, err := tx.NewUpdate().
			Model((*models.Objective)(nil)).
			Set("draft = ?", true).
			Where("id IN (?)", bun.In(adoptedIDs)).
			Exec(ctx); err != nil {
			return fmt.Errorf("drafting adopted children of %s: %w", target.ID, err)
		}
	}

	placements := make([]repositories.Placement, 0, len(remaining)+len(adopted))
	for _, obj := range append(remaining, adopted...) {
		placements = append(placements, repositories.Placement{
			ObjectiveID: obj.ID, ParentID: target.ParentID, Position: len(placements),
		})
	}
	return s.objectiveRepo.PlaceTx(ctx, tx, placements)
}

// ResetTeams clears team progress while preserving the teams themselves.
// Cannot use cascade — teams are preserved, only children are deleted.
func (s *DeleteService) ResetTeams(ctx context.Context, questID string, teamCodes []string) error {
	if len(teamCodes) == 0 {
		return nil
	}
	// Collect upload file paths before deleting records
	var allUploads []*models.Upload
	for _, runCode := range teamCodes {
		uploads, err := s.uploadsRepo.SearchByCriteria(ctx, map[string]string{
			"run_code": runCode,
		})
		if err != nil {
			return fmt.Errorf("fetching uploads for team %s: %w", runCode, err)
		}
		allUploads = append(allUploads, uploads...)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// Reset team fields (preserve the team row)
	err = s.teamRepo.Reset(ctx, tx, questID, teamCodes)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("resetting teams: %w", err)
	}

	// Delete child records explicitly (can't cascade since teams are preserved).
	// Without this, a reset run keeps its old objective completions: the
	// overview reports it as already finished, and re-proving an objective
	// hits the insert's ON CONFLICT DO NOTHING idempotency guard, so it never
	// re-fires.
	_, err = tx.NewDelete().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("run_code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting objective completions: %w", err)
	}

	_, err = tx.NewDelete().
		Model((*models.RunBlockState)(nil)).
		Where("run_code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting block states: %w", err)
	}

	_, err = tx.NewDelete().
		Model((*models.Upload)(nil)).
		Where("run_code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting uploads: %w", err)
	}

	_, err = tx.NewDelete().
		Model((*models.RunVarState)(nil)).
		Where("run_code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting team var states: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	if len(allUploads) > 0 {
		go s.cleanupUploadFiles(context.Background(), allUploads)
	}

	return nil
}

// DeleteTeams deletes teams and their associated progress data.
// Cascade handles block states, uploads, and notifications.
func (s *DeleteService) DeleteTeams(ctx context.Context, questID string, teamCodes []string) error {
	if len(teamCodes) == 0 {
		return nil
	}
	// Collect upload file paths before cascade deletes the rows
	var allUploads []*models.Upload
	for _, runCode := range teamCodes {
		uploads, err := s.uploadsRepo.SearchByCriteria(ctx, map[string]string{
			"run_code": runCode,
		})
		if err != nil {
			return fmt.Errorf("fetching uploads for team %s: %w", runCode, err)
		}
		allUploads = append(allUploads, uploads...)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// Delete teams: cascade handles block states, notifications, uploads.
	_, err = tx.NewDelete().
		Model((*models.Run)(nil)).
		Where("quest_id = ?", questID).
		Where("code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting teams: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	if len(allUploads) > 0 {
		go s.cleanupUploadFiles(context.Background(), allUploads)
	}

	return nil
}

// cleanupUploadFiles deletes physical files from the filesystem based on upload records.
// This runs in a goroutine with background context, so errors are only logged.
func (s *DeleteService) cleanupUploadFiles(ctx context.Context, uploads []*models.Upload) {
	for _, upload := range uploads {
		s.deleteUploadFile(ctx, upload.OriginalURL)

		sizes, err := upload.GetSizes()
		if err != nil {
			s.logger.WarnContext(ctx, "failed to get upload sizes", "uploadID", upload.ID, "error", err)
			continue
		}
		for _, size := range sizes {
			s.deleteUploadFile(ctx, size.URL)
		}
	}
}

// deleteUploadFile deletes a single file from the filesystem using its URL/path.
func (s *DeleteService) deleteUploadFile(ctx context.Context, urlOrPath string) {
	path := urlOrPath
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		parts := strings.SplitN(urlOrPath, "/", 4)
		//nolint:mnd // Need 4 parts to extract path after domain
		if len(parts) >= 4 {
			path = "/" + parts[3]
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) < 6 || parts[0] != "static" || parts[1] != "uploads" {
		s.logger.WarnContext(ctx, "unexpected upload URL format",
			"url", urlOrPath,
			"parsed_path", path,
			"parts", len(parts))
		return
	}

	datePath := filepath.Join(parts[2], parts[3], parts[4])
	filename := filepath.Base(path)
	filePath := filepath.Join(s.uploadsDir, datePath, filename)

	if err := os.Remove(filePath); err != nil {
		if !os.IsNotExist(err) {
			s.logger.WarnContext(ctx, "failed to delete upload file",
				"path", filePath,
				"error", err)
		}
	}
}
