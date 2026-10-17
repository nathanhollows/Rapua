package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261017000000_up, m20261017000000_down)
}

// m20261017000000_up lets a section stop being a place the player stands.
//
// Existing rows are not pass-throughs: every quest was authored when a section
// was always drawn, and defaulting the other way would change what players see.
func m20261017000000_up(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "objectives", "pass_through") {
		return nil
	}
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE "objectives" ADD COLUMN "pass_through" BOOLEAN NOT NULL DEFAULT FALSE`); err != nil {
		return fmt.Errorf("adding objectives.pass_through: %w", err)
	}
	return nil
}

// m20261017000000_down drops the column. Sections come back drawn, which is the
// only state the schema below this migration can express.
func m20261017000000_down(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "pass_through") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "pass_through"`); err != nil {
		return fmt.Errorf("dropping objectives.pass_through: %w", err)
	}
	return nil
}
