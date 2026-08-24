package repositories

import (
	"context"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

type ObjectiveRepository interface {
	GetByID(ctx context.Context, objectiveID string) (*models.Objective, error)
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
