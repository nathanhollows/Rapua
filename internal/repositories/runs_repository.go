package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// ErrUniqueConstraint is returned by InsertBatch when a run code collides with
// an existing one. Callers retry the batch with freshly generated codes.
var ErrUniqueConstraint = errors.New("unique constraint error")

type RunRepository interface {
	// InsertBatch adds multiple teams to the database
	InsertBatch(ctx context.Context, teams []models.Run) error

	// GetByCode returns a team by its code
	GetByCode(ctx context.Context, code string) (*models.Run, error)
	// GetUserIDByCode returns the user ID associated with a team code
	GetUserIDByCode(ctx context.Context, code string) (string, error)
	// CountByInstance returns the number of teams for an instance
	CountByInstance(ctx context.Context, questID string) (int, error)
	// FindAll returns all teams for an instance
	FindAll(ctx context.Context, questID string) ([]models.Run, error)

	// Update saves or updates a team in the database
	Update(ctx context.Context, t *models.Run) error
	// UpdateTeamStartedWithTx sets the team's hasStarted field to true within a transaction
	UpdateTeamStartedWithTx(ctx context.Context, tx *bun.Tx, teamID string) error
	// Reset wipes a team's progress for re-use
	Reset(ctx context.Context, tx *bun.Tx, questID string, runCodes []string) error

	// Delete removes the team from the database
	// Requires a transaction as related data will also need to be deleted
	Delete(ctx context.Context, tx *bun.Tx, questID string, runCode string) error
	// DeleteByQuestID removes all runs for a specific quest
	// Requires a transaction as this implies a cascade delete and related data
	// will also need to be deleted
	DeleteByQuestID(ctx context.Context, tx *bun.Tx, questID string) error

	// LoadQuest loads the quest for a run
	LoadQuest(ctx context.Context, team *models.Run) error
	// LoadMessages loads the messages for a team
	LoadMessages(ctx context.Context, team *models.Run) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Run) error
}

type teamRepository struct {
	db *bun.DB
}

// NewRunRepository creates a new RunRepository.
func NewRunRepository(db *bun.DB) RunRepository {
	return &teamRepository{
		db: db,
	}
}

// Update saves or updates a team in the database.
func (r *teamRepository) Update(ctx context.Context, t *models.Run) error {
	_, err := r.db.NewUpdate().Model(t).WherePK().Exec(ctx)
	return err
}

// UpdateTeamStartedWithTx sets the team's hasStarted field to true within a transaction.
func (r *teamRepository) UpdateTeamStartedWithTx(ctx context.Context, tx *bun.Tx, teamID string) error {
	_, err := tx.NewUpdate().
		Model(&models.Run{}).
		Set("has_started = ?", true).
		Set("started_at = ?", time.Now()).
		Where("code = ?", teamID).
		Exec(ctx)
	return err
}

// Reset wipes a team's progress for re-use.
func (r *teamRepository) Reset(ctx context.Context, tx *bun.Tx, questID string, runCodes []string) error {
	_, err := tx.NewUpdate().Model(&models.Run{}).
		Set("name = ''").
		Set("has_started = false").
		Set("started_at = NULL").
		Set("points = 0").
		Where("quest_id = ? AND code IN (?)", questID, bun.In(runCodes)).
		Exec(ctx)
	return err
}

func (r *teamRepository) Delete(ctx context.Context, tx *bun.Tx, questID string, runCode string) error {
	_, err := tx.
		NewDelete().
		Model(&models.Run{}).
		Where("code = ? AND quest_id = ?", runCode, questID).
		Exec(ctx)
	return err
}

func (r *teamRepository) DeleteByQuestID(ctx context.Context, tx *bun.Tx, questID string) error {
	_, err := tx.NewDelete().Model(&models.Run{}).Where("quest_id = ?", questID).Exec(ctx)
	return err
}

func (r *teamRepository) CountByInstance(ctx context.Context, questID string) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.Run)(nil)).
		Where("quest_id = ?", questID).
		Count(ctx)
	return count, err
}

func (r *teamRepository) FindAll(ctx context.Context, questID string) ([]models.Run, error) {
	var teams []models.Run
	err := r.db.NewSelect().
		Model(&teams).
		Where("run.quest_id = ?", questID).
		Scan(ctx)
	if err != nil {
		return teams, err
	}
	return teams, nil
}

// GetByCode returns a team by code.
func (r *teamRepository) GetByCode(ctx context.Context, code string) (*models.Run, error) {
	code = strings.ToUpper(code)
	var team models.Run
	err := r.db.NewSelect().Model(&team).Where("run.code = ?", code).
		Relation("Quest").
		Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("FindTeamByCode: %w", err)
	}
	return &team, nil
}

// GetUserIDByCode returns the user ID associated with a team code.
func (r *teamRepository) GetUserIDByCode(ctx context.Context, code string) (string, error) {
	code = strings.ToUpper(code)
	var quest models.Quest

	q := r.db.NewSelect().
		Model(&quest).
		Column("quest.user_id").
		Join("JOIN runs AS run ON run.quest_id = quest.id").
		Where("run.code = ?", code)
	err := q.Scan(ctx)
	if err != nil {
		return "", fmt.Errorf("GetUserIDByCode: %w", err)
	}

	return quest.UserID, nil
}

// InsertBatch inserts a batch of teams and returns an error if there's a unique constraint conflict.
func (r *teamRepository) InsertBatch(ctx context.Context, teams []models.Run) error {
	for teamIndex := range teams {
		if teams[teamIndex].ID == "" {
			teams[teamIndex].ID = uuid.New().String()
		}
	}
	_, err := r.db.NewInsert().Model(&teams).Exec(ctx)
	if err != nil && isUniqueConstraintError(err) {
		return ErrUniqueConstraint
	}
	return err
}

// isUniqueConstraintError checks if an error is due to a unique constraint violation.
func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unique constraint")
}

func (r *teamRepository) LoadQuest(ctx context.Context, team *models.Run) error {
	query := r.db.NewSelect().
		Model(&team.Quest).
		Where("id = ?", team.QuestID).
		WherePK()

	if team.Quest.Settings.QuestID == "" {
		query = query.Relation("Settings")
	}

	return query.Scan(ctx)
}

func (r *teamRepository) LoadMessages(ctx context.Context, team *models.Run) error {
	err := r.db.NewSelect().Model(&team.Messages).
		Where("run_code = ?", team.Code).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("LoadNotifications: %w", err)
	}
	return nil
}

func (r *teamRepository) LoadRelations(ctx context.Context, team *models.Run) error {
	err := r.LoadQuest(ctx, team)
	if err != nil {
		return err
	}

	err = r.LoadMessages(ctx, team)
	if err != nil {
		return err
	}

	return nil
}
