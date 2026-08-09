package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS "spaces" (
					"id"         VARCHAR NOT NULL PRIMARY KEY,
					"quest_id"   VARCHAR NOT NULL REFERENCES "quests"("id") ON DELETE CASCADE,
					"slug"       VARCHAR(255) NOT NULL,
					"name"       VARCHAR(255) NOT NULL,
					"kind"       VARCHAR(32) NOT NULL,
					"geometry"   TEXT,
					"payload"    VARCHAR(255),
					"mobile"     BOOLEAN NOT NULL DEFAULT FALSE,
					"created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					"updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`); err != nil {
				return fmt.Errorf("create spaces: %w", err)
			}

			// Slugs address a space within its quest.
			if _, err := db.ExecContext(ctx, `
				CREATE UNIQUE INDEX IF NOT EXISTS "idx_spaces_quest_slug" ON "spaces" ("quest_id", "slug")
			`); err != nil {
				return fmt.Errorf("create spaces slug index: %w", err)
			}

			// A scan resolves without knowing the quest, so a payload must identify
			// one space table-wide. Partial, because an unset payload is absence
			// rather than a value that can collide.
			if _, err := db.ExecContext(ctx, `
				CREATE UNIQUE INDEX IF NOT EXISTS "idx_spaces_payload" ON "spaces" ("payload")
				WHERE "payload" != ''
			`); err != nil {
				return fmt.Errorf("create spaces payload index: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "spaces"`)
			return err
		},
	)
}
