package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Adds the FK constraints 20260822000000_objectives.go's plain columns were
// missing, matching the pattern locations/check_ins already use (recreateTable,
// since SQLite cannot ALTER TABLE ADD CONSTRAINT). Without these, deleting a
// quest or run left objectives, and objective_context_completions, orphaned:
// objectives.quest_id and objective_context_completions.run_code/objective_id
// had no FK at all, unlike every other quest/run-owned table.
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(
				ctx,
				`DELETE FROM "objectives" WHERE NOT EXISTS
					(SELECT 1 FROM "quests" WHERE "quests"."id" = "objectives"."quest_id")`,
			)
			if err != nil {
				return fmt.Errorf("pruning orphaned objectives: %w", err)
			}
			_, err = db.ExecContext(
				ctx,
				`DELETE FROM "objective_context_completions" WHERE NOT EXISTS
					(SELECT 1 FROM "runs" WHERE "runs"."code" = "objective_context_completions"."run_code")
					OR NOT EXISTS
					(SELECT 1 FROM "objectives" WHERE "objectives"."id" = "objective_context_completions"."objective_id")`,
			)
			if err != nil {
				return fmt.Errorf("pruning orphaned objective_context_completions: %w", err)
			}

			err = recreateTable(ctx, db, "objectives", `CREATE TABLE "objectives" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"quest_id" VARCHAR(36) NOT NULL REFERENCES "quests"("id") ON DELETE CASCADE,
				"slug" VARCHAR(255),
				"title" VARCHAR(255),
				"when_clause" TEXT,
				"order" INT,
				"proof_sets" TEXT,
				"reveal_sets" TEXT
			)`)
			if err != nil {
				return fmt.Errorf("recreating objectives with FK: %w", err)
			}
			if _, err := db.ExecContext(
				ctx,
				`CREATE INDEX IF NOT EXISTS "idx_objectives_quest_id" ON "objectives" ("quest_id")`,
			); err != nil {
				return fmt.Errorf("recreating idx_objectives_quest_id: %w", err)
			}
			if _, err := db.ExecContext(
				ctx,
				`CREATE UNIQUE INDEX IF NOT EXISTS "objectives_quest_slug" ON "objectives" ("quest_id", "slug") WHERE "slug" != ''`,
			); err != nil {
				return fmt.Errorf("recreating objectives_quest_slug: %w", err)
			}

			err = recreateTable(
				ctx, db, "objective_context_completions",
				`CREATE TABLE "objective_context_completions" (
				"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
				"objective_id" VARCHAR(36) NOT NULL REFERENCES "objectives"("id") ON DELETE CASCADE,
				"context" VARCHAR(32) NOT NULL,
				"completed_at" DATETIME NOT NULL DEFAULT current_timestamp,
				PRIMARY KEY ("run_code", "objective_id", "context")
			)`)
			if err != nil {
				return fmt.Errorf("recreating objective_context_completions with FK: %w", err)
			}
			if _, err := db.ExecContext(
				ctx,
				`CREATE INDEX IF NOT EXISTS "idx_objective_context_completions_run_code" `+
					`ON "objective_context_completions" ("run_code")`,
			); err != nil {
				return fmt.Errorf("recreating idx_objective_context_completions_run_code: %w", err)
			}

			return nil
		},
		func(_ context.Context, _ *bun.DB) error {
			// Dropping the FK constraints back out would require the same
			// recreate-table dance in reverse; not worth supporting since the
			// constraints only make invalid states impossible, they don't change
			// any data shape a rollback would need to undo.
			return nil
		},
	)
}
