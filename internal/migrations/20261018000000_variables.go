package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261018000000_up, m20261018000000_down)
}

// m20261018000000_up lets an objective wait on variables and records which ones
// each run has set.
//
// A run's variables are stored rather than read back from its block states:
// a block in an objective the run could not reach still saves its state, but
// earns nothing, and a gate opened that way would let a player skip ahead.
// block_id carries no constraint for the reason run_events gives: a block an
// author deletes should not take back a gate it already opened.
func m20261018000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "depends") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" ADD COLUMN "depends" TEXT`); err != nil {
			return fmt.Errorf("adding objectives.depends: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "run_variables" (
		"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"name" VARCHAR(64) NOT NULL,
		"block_id" VARCHAR(36),
		"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
		PRIMARY KEY ("run_code", "name")
	)`); err != nil {
		return fmt.Errorf("creating run_variables: %w", err)
	}
	return nil
}

// m20261018000000_down drops both. A run's variables cannot be rebuilt from
// anything else, so a down loses which gates each run had opened.
func m20261018000000_down(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "run_variables"`); err != nil {
		return fmt.Errorf("dropping run_variables: %w", err)
	}
	if columnExists(ctx, db, "objectives", "depends") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "depends"`); err != nil {
			return fmt.Errorf("dropping objectives.depends: %w", err)
		}
	}
	return nil
}
