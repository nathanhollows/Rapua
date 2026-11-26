package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261011000000_up, m20261011000000_down)
}

// m20261011000000_up leaves objective_id and block_id unconstrained: deleting
// an objective must not erase the record of teams having visited it.
func m20261011000000_up(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "run_events" (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"quest_id" VARCHAR(36) NOT NULL REFERENCES "quests"("id") ON DELETE CASCADE,
		"type" VARCHAR(48) NOT NULL,
		"objective_id" VARCHAR(36),
		"block_id" VARCHAR(36),
		"points" INTEGER NOT NULL DEFAULT 0,
		"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"data" JSONB
	)`); err != nil {
		return fmt.Errorf("creating run_events: %w", err)
	}

	// Streams are read by run or by quest, in time order.
	if _, err := db.ExecContext(
		ctx,
		`CREATE INDEX IF NOT EXISTS "idx_run_events_run_code_created_at" `+
			`ON "run_events" ("run_code", "created_at")`,
	); err != nil {
		return fmt.Errorf("indexing run_events.run_code: %w", err)
	}

	if _, err := db.ExecContext(
		ctx,
		`CREATE INDEX IF NOT EXISTS "idx_run_events_quest_id_created_at" `+
			`ON "run_events" ("quest_id", "created_at")`,
	); err != nil {
		return fmt.Errorf("indexing run_events.quest_id: %w", err)
	}

	return nil
}

// m20261011000000_down loses history nothing else can rebuild.
func m20261011000000_down(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS "idx_run_events_quest_id_created_at"`); err != nil {
		return fmt.Errorf("dropping run_events.quest_id index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS "idx_run_events_run_code_created_at"`); err != nil {
		return fmt.Errorf("dropping run_events.run_code index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "run_events"`); err != nil {
		return fmt.Errorf("dropping run_events: %w", err)
	}
	return nil
}
