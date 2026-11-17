package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260822000000_up, m20260822000000_down)
}

// m20260822000000_statements create the final shape directly: production never
// held any of it.
//
// parent_id has no self-reference: the repository reparents and deletes in
// steps a cascade would interrupt.
//
// (parent_id, position) is unique because a tie breaks by id and silently
// reorders an ordered quest. NULL roots are distinct, so roots never collide.
//
// run_variables.block_id has no FK: deleting a block must not close a gate it
// opened.
var m20260822000000_statements = []struct{ name, sql string }{
	{"objectives", `CREATE TABLE IF NOT EXISTS "objectives" (
		"id" VARCHAR(36) NOT NULL PRIMARY KEY,
		"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"quest_id" VARCHAR(36) NOT NULL REFERENCES "quests"("id") ON DELETE CASCADE,
		"slug" VARCHAR(255),
		"title" VARCHAR(255),
		"description" TEXT NOT NULL DEFAULT '',
		"parent_id" VARCHAR(36),
		"position" INT NOT NULL DEFAULT 0,
		"routing" VARCHAR(32),
		"children_min" INT,
		"children_max" INT,
		"max_next" INT NOT NULL DEFAULT 0,
		"finish_label" VARCHAR(255),
		"draft" BOOLEAN NOT NULL DEFAULT FALSE,
		"pass_through" BOOLEAN NOT NULL DEFAULT FALSE,
		"depends" TEXT
	)`},
	{"idx_objectives_quest_id", `CREATE INDEX IF NOT EXISTS "idx_objectives_quest_id" ON "objectives" ("quest_id")`},
	{"objectives_quest_slug", `CREATE UNIQUE INDEX IF NOT EXISTS "objectives_quest_slug"` +
		` ON "objectives" ("quest_id", "slug") WHERE "slug" != ''`},
	{"idx_objectives_parent_id", `CREATE INDEX IF NOT EXISTS "idx_objectives_parent_id" ON "objectives" ("parent_id")`},
	{"idx_objectives_parent_position", `CREATE UNIQUE INDEX IF NOT EXISTS "idx_objectives_parent_position"` +
		` ON "objectives" ("parent_id", "position")`},
	{"objective_context_completions", `CREATE TABLE IF NOT EXISTS "objective_context_completions" (
		"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"objective_id" VARCHAR(36) NOT NULL REFERENCES "objectives"("id") ON DELETE CASCADE,
		"context" VARCHAR(32) NOT NULL,
		"completed_at" DATETIME NOT NULL DEFAULT current_timestamp,
		PRIMARY KEY ("run_code", "objective_id", "context")
	)`},
	{"idx_objective_context_completions_run_code", `CREATE INDEX IF NOT EXISTS` +
		` "idx_objective_context_completions_run_code" ON "objective_context_completions" ("run_code")`},
	// run_code leads the primary key, so it needs no index.
	{"section_finishes", `CREATE TABLE IF NOT EXISTS "section_finishes" (
		"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"objective_id" VARCHAR(36) NOT NULL REFERENCES "objectives"("id") ON DELETE CASCADE,
		"finished_at" DATETIME NOT NULL DEFAULT current_timestamp,
		PRIMARY KEY ("run_code", "objective_id")
	)`},
	{"run_variables", `CREATE TABLE IF NOT EXISTS "run_variables" (
		"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"name" VARCHAR(64) NOT NULL,
		"block_id" VARCHAR(36),
		"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
		PRIMARY KEY ("run_code", "name")
	)`},
}

// m20260822000000_up is guarded throughout, so a rerun after a partial failure
// works.
func m20260822000000_up(ctx context.Context, db *bun.DB) error {
	for _, stmt := range m20260822000000_statements {
		if _, err := db.ExecContext(ctx, stmt.sql); err != nil {
			return fmt.Errorf("creating %s: %w", stmt.name, err)
		}
	}
	return nil
}

func m20260822000000_down(ctx context.Context, db *bun.DB) error {
	for _, table := range []string{
		"run_variables", "section_finishes", "objective_context_completions", "objectives",
	} {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %q`, table)); err != nil {
			return fmt.Errorf("dropping %s: %w", table, err)
		}
	}
	return nil
}
