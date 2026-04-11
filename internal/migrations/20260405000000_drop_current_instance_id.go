package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx,
				`ALTER TABLE "users" DROP COLUMN "current_instance_id"`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx,
				`ALTER TABLE "users" ADD COLUMN "current_instance_id" VARCHAR(36)`)
			return err
		},
	)
}
