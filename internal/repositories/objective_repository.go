package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

type ObjectiveRepository interface {
	GetByID(ctx context.Context, objectiveID string) (*models.Objective, error)
	GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error)
	FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error)
	CreateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error
	UpdateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error
	Delete(ctx context.Context, tx *bun.Tx, objectiveID string) error
	DeleteByQuestID(ctx context.Context, tx *bun.Tx, questID string) error
}

type objectiveRepository struct {
	db *bun.DB
}

func NewObjectiveRepository(db *bun.DB) ObjectiveRepository {
	return &objectiveRepository{db: db}
}

func (r *objectiveRepository) GetByID(ctx context.Context, objectiveID string) (*models.Objective, error) {
	var objective models.Objective
	err := r.db.NewSelect().
		Model(&objective).
		Where("id = ?", objectiveID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objective: %w", err)
	}
	return &objective, nil
}

func (r *objectiveRepository) GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error) {
	var objective models.Objective
	err := r.db.NewSelect().
		Model(&objective).
		Where("quest_id = ? AND slug = ?", questID, slug).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objective: %w", err)
	}
	return &objective, nil
}

// FindByQuestID: order comes from the structure tree's ObjectiveIDs array, not
// this column: nothing writes Objective.Order (same as Location).
func (r *objectiveRepository) FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error) {
	var objectives []models.Objective
	err := r.db.NewSelect().
		Model(&objectives).
		Where("quest_id = ?", questID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objectives for quest: %w", err)
	}
	return objectives, nil
}

func (r *objectiveRepository) CreateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error {
	if objective.ID == "" {
		objective.ID = uuid.New().String()
	}
	_, err := tx.NewInsert().Model(objective).Exec(ctx)
	return err
}

func (r *objectiveRepository) UpdateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error {
	_, err := tx.NewUpdate().Model(objective).WherePK().Exec(ctx)
	return err
}

func (r *objectiveRepository) Delete(ctx context.Context, tx *bun.Tx, objectiveID string) error {
	_, err := tx.NewDelete().Model(&models.Objective{ID: objectiveID}).WherePK().Exec(ctx)
	return err
}

func (r *objectiveRepository) DeleteByQuestID(ctx context.Context, tx *bun.Tx, questID string) error {
	_, err := tx.NewDelete().Model(&models.Objective{}).Where("quest_id = ?", questID).Exec(ctx)
	return err
}
