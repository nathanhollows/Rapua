package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260921000000_up, m20260921000000_down)
}

// m20260921000000_up drops the conditional-visibility system: an objective's
// depends list, the sets each context wrote, and the run variables those two
// passed between them.
//
// Depends was the only reader of the variables, so removing it left sets
// writing to nothing and every var lint rule reporting on a system with no
// effect. It goes whole rather than in halves, and comes back the same way.
//
// Columns are dropped rather than left unmapped: an unmapped column invites a
// later reader to wonder whether the feature is still there.
func m20260921000000_up(ctx context.Context, db *bun.DB) error {
	for _, column := range []string{"depends", "proof_sets", "reveal_sets"} {
		if !columnExists(ctx, db, "objectives", column) {
			continue
		}
		if _, err := db.ExecContext(
			ctx, fmt.Sprintf(`ALTER TABLE "objectives" DROP COLUMN %q`, column),
		); err != nil {
			return fmt.Errorf("dropping objectives.%s: %w", column, err)
		}
	}

	if tableExists(ctx, db, "run_var_states") {
		if _, err := db.ExecContext(ctx, `DROP TABLE "run_var_states"`); err != nil {
			return fmt.Errorf("dropping run_var_states: %w", err)
		}
	}

	return nil
}

// m20260921000000_down restores the shape, not the content. The gates these
// held are gone from the spec, so there is nothing to put back into them.
func m20260921000000_down(ctx context.Context, db *bun.DB) error {
	for _, column := range []string{"depends", "proof_sets", "reveal_sets"} {
		if columnExists(ctx, db, "objectives", column) {
			continue
		}
		if _, err := db.ExecContext(
			ctx, fmt.Sprintf(`ALTER TABLE "objectives" ADD COLUMN %q TEXT`, column),
		); err != nil {
			return fmt.Errorf("restoring objectives.%s: %w", column, err)
		}
	}
	return nil
}
