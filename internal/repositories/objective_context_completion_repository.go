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
	// InsertWithTx is Insert inside a caller's transaction, so the completion and
	// the event recording it commit or roll back together.
	InsertWithTx(
		ctx context.Context,
		tx *bun.Tx,
		runCode, objectiveID string,
		blockContext game.BlockContext,
	) (inserted bool, err error)
	FindCompletedObjectiveIDs(
		ctx context.Context,
		runCode string,
		blockContext game.BlockContext,
	) ([]string, error)
	// FindCompletedByQuest avoids a scan per run when a whole quest is read.
	FindCompletedByQuest(
		ctx context.Context, questID string, blockContext game.BlockContext,
	) (map[string][]string, error)
	// FindCompletedObjectiveIDsOrdered is FindCompletedObjectiveIDs ordered by
	// completion time, most recent first, for the player's /journal.
	FindCompletedObjectiveIDsOrdered(
		ctx context.Context,
		runCode string,
		blockContext game.BlockContext,
	) ([]string, error)
	CountCompletedObjectivesByRun(
		ctx context.Context,
		questID string,
		blockContext game.BlockContext,
	) (map[string]int, error)
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
	return insertCompletion(ctx, r.db, runCode, objectiveID, blockContext)
}

func (r *objectiveContextCompletionRepository) InsertWithTx(
	ctx context.Context,
	tx *bun.Tx,
	runCode, objectiveID string,
	blockContext game.BlockContext,
) (bool, error) {
	return insertCompletion(ctx, tx, runCode, objectiveID, blockContext)
}

// insertCompletion serves both callers: the conflict clause is the guard whether
// it runs inside a transaction or not.
func insertCompletion(
	ctx context.Context,
	db bun.IDB,
	runCode, objectiveID string,
	blockContext game.BlockContext,
) (bool, error) {
	row := &models.ObjectiveContextCompletion{
		RunCode:     runCode,
		ObjectiveID: objectiveID,
		Context:     blockContext,
	}
	res, err := db.NewInsert().
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

func (r *objectiveContextCompletionRepository) FindCompletedByQuest(
	ctx context.Context, questID string, blockContext game.BlockContext,
) (map[string][]string, error) {
	var rows []runObjectiveRow
	// The log has no quest column, so go through the objective.
	err := r.db.NewSelect().
		TableExpr("objective_context_completions AS occ").
		ColumnExpr("occ.run_code AS run_code").
		ColumnExpr("occ.objective_id AS objective_id").
		Join("JOIN objectives AS o ON o.id = occ.objective_id").
		Where("o.quest_id = ? AND occ.context = ?", questID, blockContext).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	return groupByRun(rows), nil
}

func (r *objectiveContextCompletionRepository) FindCompletedObjectiveIDs(
	ctx context.Context,
	runCode string,
	blockContext game.BlockContext,
) ([]string, error) {
	var ids []string
	err := r.db.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Column("objective_id").
		Where("run_code = ? AND context = ?", runCode, blockContext).
		Scan(ctx, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *objectiveContextCompletionRepository) FindCompletedObjectiveIDsOrdered(
	ctx context.Context,
	runCode string,
	blockContext game.BlockContext,
) ([]string, error) {
	var ids []string
	err := r.db.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Column("objective_id").
		Where("run_code = ? AND context = ?", runCode, blockContext).
		Order("completed_at DESC").
		Scan(ctx, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// CountCompletedObjectivesByRun replaces run.CheckIns as the source of a
// run's progress count: check-ins are no longer written once a quest is
// objective-built, so anything counting them stays at zero forever.
func (r *objectiveContextCompletionRepository) CountCompletedObjectivesByRun(
	ctx context.Context,
	questID string,
	blockContext game.BlockContext,
) (map[string]int, error) {
	var rows []struct {
		RunCode string `bun:"run_code"`
		Count   int    `bun:"count"`
	}
	err := r.db.NewSelect().
		TableExpr("objective_context_completions AS occ").
		ColumnExpr("occ.run_code AS run_code").
		ColumnExpr("COUNT(*) AS count").
		Join("JOIN objectives AS o ON o.id = occ.objective_id").
		Where("o.quest_id = ? AND occ.context = ?", questID, blockContext).
		Group("occ.run_code").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.RunCode] = row.Count
	}
	return counts, nil
}
