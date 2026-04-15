package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS "team_var_states" (
					"team_code"   VARCHAR NOT NULL REFERENCES "teams"("code") ON DELETE CASCADE,
					"instance_id" VARCHAR NOT NULL,
					"var_name"    VARCHAR NOT NULL,
					"var_value"   VARCHAR NOT NULL,
					"updated_at"  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY ("team_code", "instance_id", "var_name")
				)
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "team_var_states"`)
			return err
		},
	)
}
