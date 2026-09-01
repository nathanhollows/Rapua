package repositories

import (
	"context"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// SectionFinishRepository manages the append-only log of players ending a
// section. It mirrors ObjectiveContextCompletionRepository: rows are only ever
// inserted, and the primary key is the idempotency guard.
type SectionFinishRepository interface {
	// Insert records that a run finished a section. inserted is true only when
	// this call is the one that recorded it: false means the press was already
	// logged, which is the caller's guard against acting on it twice.
	Insert(ctx context.Context, runCode, objectiveID string) (inserted bool, err error)
	// FindFinishedObjectiveIDs returns every objective this run has finished.
	FindFinishedObjectiveIDs(ctx context.Context, runCode string) ([]string, error)
}

type sectionFinishRepository struct {
	db *bun.DB
}

func NewSectionFinishRepository(db *bun.DB) SectionFinishRepository {
	return &sectionFinishRepository{db: db}
}

func (r *sectionFinishRepository) Insert(
	ctx context.Context, runCode, objectiveID string,
) (bool, error) {
	row := &models.SectionFinish{
		RunCode:     runCode,
		ObjectiveID: objectiveID,
	}
	res, err := r.db.NewInsert().
		Model(row).
		On("CONFLICT (run_code, objective_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("logging section finish: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("counting inserted section finishes: %w", err)
	}
	return affected > 0, nil
}

func (r *sectionFinishRepository) FindFinishedObjectiveIDs(
	ctx context.Context, runCode string,
) ([]string, error) {
	var ids []string
	err := r.db.NewSelect().
		Model((*models.SectionFinish)(nil)).
		Column("objective_id").
		Where("run_code = ?", runCode).
		Order("finished_at ASC", "objective_id ASC").
		Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("finding finished sections: %w", err)
	}
	return ids, nil
}
