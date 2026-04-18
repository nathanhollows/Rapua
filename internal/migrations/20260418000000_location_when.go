package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE locations ADD COLUMN "when_clause" TEXT NULL DEFAULT NULL
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE locations DROP COLUMN "when_clause"
			`)
			return err
		},
	)
}
