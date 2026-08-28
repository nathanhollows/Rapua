// Package services provides entity deletion with transaction safety.
// Relies on ON DELETE CASCADE constraints at the database level to handle
// most child record cleanup. Application code explicitly deletes blocks
// (whose owner_id is polymorphic and cannot carry a FK constraint) and
// handles upload file cleanup, unused marker cleanup, and location statistics.
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

// DeleteService coordinates deletions. Database-level ON DELETE CASCADE
// handles child record cleanup; this service handles auth, file cleanup,
// and denormalized data updates.
type DeleteService struct {
	transactor   db.Transactor
	instanceRepo repositories.QuestRepository
	locationRepo repositories.LocationRepository
	markerRepo   repositories.MarkerRepository
	teamRepo     repositories.RunRepository
	uploadsRepo  repositories.UploadsRepository
	db           *bun.DB
	uploadsDir   string
	logger       *slog.Logger
}

// NewDeleteService creates a new DeleteService with the provided dependencies.
func NewDeleteService(
	transactor db.Transactor,
	instanceRepo repositories.QuestRepository,
	locationRepo repositories.LocationRepository,
	markerRepo repositories.MarkerRepository,
	teamRepo repositories.RunRepository,
	uploadsRepo repositories.UploadsRepository,
	db *bun.DB,
	uploadsDir string,
	logger *slog.Logger,
) *DeleteService {
	return &DeleteService{
		transactor:   transactor,
		instanceRepo: instanceRepo,
		locationRepo: locationRepo,
		markerRepo:   markerRepo,
		teamRepo:     teamRepo,
		uploadsRepo:  uploadsRepo,
		db:           db,
		uploadsDir:   uploadsDir,
		logger:       logger,
	}
}

// DeleteUser deletes a user and all associated data.
// Cascade handles: instances, locations, teams, check-ins, states,
// settings, credits, purchases, start logs, notifications, facilitator tokens.
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

	// blocks.owner_id has no FK; delete blocks for all locations and objectives
	// owned by this user's quests, then delete the quest-level (start/finish) blocks.
	_, err = tx.NewDelete().Model((*models.Block)(nil)).
		Where("owner_id IN (SELECT l.id FROM locations l JOIN quests q ON l.quest_id = q.id WHERE q.user_id = ?)", userID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting location blocks for user: %w", err)
	}
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

	// Delete location-owned, objective-owned, then instance-owned (start/finish) blocks.
	// blocks.owner_id has no FK so they won't be cascade-deleted automatically.
	_, err = tx.NewDelete().Model((*models.Block)(nil)).
		Where("owner_id IN (SELECT id FROM locations WHERE quest_id = ?)", questID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting location blocks: %w", err)
	}
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

// DeleteLocation deletes a location and its blocks, then cleans up unused markers.
// blocks.owner_id has no FK so blocks must be deleted explicitly.
func (s *DeleteService) DeleteLocation(ctx context.Context, locationID string) error {
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

	// GameStructure is a JSON blob with no FK, so nothing else drops the ID.
	if err := s.pruneLocationFromStructure(ctx, tx, locationID); err != nil {
		_ = tx.Rollback()
		return err
	}

	// Delete blocks first: cascade handles run_block_states.
	_, err = tx.NewDelete().
		Model((*models.Block)(nil)).
		Where("owner_id = ?", locationID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting blocks for location: %w", err)
	}

	// Delete location — cascade handles check_ins and any remaining children
	_, err = tx.NewDelete().
		Model((*models.Location)(nil)).
		Where("id = ?", locationID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting location: %w", err)
	}

	// Clean up markers that are no longer referenced by any location
	err = s.markerRepo.DeleteUnused(ctx, tx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting unused markers: %w", err)
	}

	return tx.Commit()
}

// DeleteObjective: blocks.owner_id has no FK, so blocks are deleted
// explicitly.
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

	// GameStructure is a JSON blob with no FK, so nothing else drops the ID.
	if err := s.pruneObjectiveFromStructure(ctx, tx, objectiveID); err != nil {
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

	// Delete objective: cascade handles any remaining children.
	_, err = tx.NewDelete().
		Model((*models.Objective)(nil)).
		Where("id = ?", objectiveID).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting objective: %w", err)
	}

	return tx.Commit()
}

// pruneObjectiveFromStructure tolerates a missing objective or quest so that
// DeleteObjective stays idempotent.
func (s *DeleteService) pruneObjectiveFromStructure(
	ctx context.Context,
	tx *bun.Tx,
	objectiveID string,
) error {
	var objective models.Objective
	err := tx.NewSelect().
		Model(&objective).
		Column("id", "quest_id").
		Where("id = ?", objectiveID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("loading objective for structure prune: %w", err)
	}

	var quest models.Quest
	err = tx.NewSelect().
		Model(&quest).
		Where("id = ?", objective.QuestID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("loading quest for structure prune: %w", err)
	}

	if !quest.GameStructure.RemoveObjectiveID(objectiveID) {
		return nil
	}

	_, err = tx.NewUpdate().
		Model(&quest).
		Column("game_structure").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("saving pruned structure: %w", err)
	}

	return nil
}

// pruneLocationFromStructure tolerates a missing location or quest so that
// DeleteLocation stays idempotent.
func (s *DeleteService) pruneLocationFromStructure(
	ctx context.Context,
	tx *bun.Tx,
	locationID string,
) error {
	var location models.Location
	err := tx.NewSelect().
		Model(&location).
		Column("id", "quest_id").
		Where("id = ?", locationID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("loading location for structure prune: %w", err)
	}

	var quest models.Quest
	err = tx.NewSelect().
		Model(&quest).
		Where("id = ?", location.QuestID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("loading quest for structure prune: %w", err)
	}

	if !quest.GameStructure.RemoveLocationID(locationID) {
		return nil
	}

	_, err = tx.NewUpdate().
		Model(&quest).
		Column("game_structure").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("saving pruned structure: %w", err)
	}

	return nil
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

	// Delete child records explicitly (can't cascade since teams are preserved)
	_, err = tx.NewDelete().
		Model((*models.CheckIn)(nil)).
		Where("quest_id = ?", questID).
		Where("run_code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting check-ins: %w", err)
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

	err = s.locationRepo.UpdateStatistics(ctx, tx, questID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("updating location statistics: %w", err)
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
// Cascade handles check-ins, block states, uploads, and notifications.
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

	// Delete teams — cascade handles check-ins, block states, notifications, uploads
	_, err = tx.NewDelete().
		Model((*models.Run)(nil)).
		Where("quest_id = ?", questID).
		Where("code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting teams: %w", err)
	}

	// Update denormalized location statistics
	err = s.locationRepo.UpdateStatistics(ctx, tx, questID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("updating location statistics: %w", err)
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
