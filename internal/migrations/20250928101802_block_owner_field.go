package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Rename location_id column to owner_id in blocks table
		_, err := db.ExecContext(ctx, "ALTER TABLE blocks RENAME COLUMN location_id TO owner_id")
		if err != nil {
			return fmt.Errorf("failed to rename location_id to owner_id: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Rename owner_id column back to location_id in blocks table
		_, err := db.ExecContext(ctx, "ALTER TABLE blocks RENAME COLUMN owner_id TO location_id")
		if err != nil {
			return fmt.Errorf("failed to rename owner_id back to location_id: %w", err)
		}

		return nil
	})
}
