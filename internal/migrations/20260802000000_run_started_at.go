package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// created_at is when a run was provisioned, which is not when players began it.
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if !tableExists(ctx, db, "runs") || columnExists(ctx, db, "runs", "started_at") {
				return nil
			}
			if _, err := db.ExecContext(ctx, `ALTER TABLE runs ADD COLUMN started_at TIMESTAMP`); err != nil {
				return fmt.Errorf("add started_at to runs: %w", err)
			}
			// No start was recorded for these, and a started run must not read null.
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
