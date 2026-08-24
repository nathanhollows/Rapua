package repositories

import (
	"context"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// ObjectiveContextCompletionRepository manages the append-only objective
// context completion log.
type ObjectiveContextCompletionRepository interface {
	// Insert records a context as complete for a run. inserted is true only when this
	// call is the one that recorded it for the first time; false means it was already
	// logged: the caller's idempotency guard.
	Insert(
		ctx context.Context,
		runCode, objectiveID string,
		blockContext game.BlockContext,
	) (inserted bool, err error)
}

type objectiveContextCompletionRepository struct {
	db *bun.DB
}

func NewObjectiveContextCompletionRepository(db *bun.DB) ObjectiveContextCompletionRepository {
	return &objectiveContextCompletionRepository{db: db}
}

// Insert no-ops when the context was already logged.
func (r *objectiveContextCompletionRepository) Insert(
	ctx context.Context,
	runCode, objectiveID string,
	blockContext game.BlockContext,
) (bool, error) {
	row := &models.ObjectiveContextCompletion{
		RunCode:     runCode,
		ObjectiveID: objectiveID,
		Context:     blockContext,
	}
	res, err := r.db.NewInsert().
		Model(row).
		On("CONFLICT (run_code, objective_id, context) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
