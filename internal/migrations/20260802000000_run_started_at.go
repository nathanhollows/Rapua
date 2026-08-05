package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Adds runs.started_at — when the players began the run, as distinct from
// created_at (when the run was provisioned). Backs the run.started_at built-in.
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if !tableExists(ctx, db, "runs") || columnExists(ctx, db, "runs", "started_at") {
				return nil
			}
			if _, err := db.ExecContext(ctx, `ALTER TABLE runs ADD COLUMN started_at TIMESTAMP`); err != nil {
				return fmt.Errorf("add started_at to runs: %w", err)
			}
			// Runs already underway have no recorded start; fall back to when the
			// run row was created so the var is never null for a started run.
			if _, err := db.ExecContext(ctx,
				`UPDATE runs SET started_at = created_at WHERE has_started = true AND started_at IS NULL`,
			); err != nil {
				return fmt.Errorf("backfill started_at: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			if !tableExists(ctx, db, "runs") || !columnExists(ctx, db, "runs", "started_at") {
				return nil
			}
			if _, err := db.ExecContext(ctx, `ALTER TABLE runs DROP COLUMN started_at`); err != nil {
				return fmt.Errorf("drop started_at from runs: %w", err)
			}
			return nil
		},
	)
}
