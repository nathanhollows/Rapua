package repositories

import (
	"context"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// RunVariableRepository records the variables each run has set.
type RunVariableRepository interface {
	// InsertWithTx sets names for a run inside the caller's transaction, so a
	// variable is set only if the answer that earned it commits. It returns the
	// names this call set for the first time; a name already set is left with
	// the block that set it first.
	InsertWithTx(ctx context.Context, tx *bun.Tx, runCode, blockID string, names []string) ([]string, error)
	// FindNamesByRunCode returns every variable the run has set.
	FindNamesByRunCode(ctx context.Context, runCode string) ([]string, error)
}

type runVariableRepository struct {
	db *bun.DB
}

// NewRunVariableRepository creates a RunVariableRepository.
func NewRunVariableRepository(db *bun.DB) RunVariableRepository {
	return &runVariableRepository{db: db}
}

func (r *runVariableRepository) InsertWithTx(
	ctx context.Context,
	tx *bun.Tx,
	runCode, blockID string,
	names []string,
) ([]string, error) {
	var set []string
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		res, err := tx.NewInsert().
			Model(&models.RunVariable{RunCode: runCode, Name: name, BlockID: blockID}).
			On("CONFLICT (run_code, name) DO NOTHING").
			Exec(ctx)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected > 0 {
			set = append(set, name)
		}
	}
	return set, nil
}

func (r *runVariableRepository) FindNamesByRunCode(ctx context.Context, runCode string) ([]string, error) {
	var names []string
	err := r.db.NewSelect().
		Model((*models.RunVariable)(nil)).
		Column("name").
		Where("run_code = ?", runCode).
		Order("name").
		Scan(ctx, &names)
	return names, err
}
