package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE locations DROP COLUMN content_id`)
		if err != nil {
			return fmt.Errorf("dropping content_id column: %w", err)
		}

		_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS location_contents`)
		if err != nil {
			return fmt.Errorf("dropping location_contents table: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE locations ADD COLUMN content_id VARCHAR(255) NOT NULL DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("adding content_id column: %w", err)
		}

		_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS location_contents (
			id VARCHAR(36) PRIMARY KEY,
			content TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
		if err != nil {
			return fmt.Errorf("recreating location_contents table: %w", err)
		}

		return nil
	})
}
