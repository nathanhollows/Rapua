package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261004000000_up, m20261004000000_down)
}

// Colour now encodes the routing strategy, so the stored value has no reader.
func m20261004000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "color") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "color"`); err != nil {
		return fmt.Errorf("dropping objectives.color: %w", err)
	}
	return nil
}

// The down migration restores the column only; the old values are not recoverable.
func m20261004000000_down(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "objectives", "color") {
		return nil
	}
	if _, err := db.ExecContext(
		ctx, `ALTER TABLE "objectives" ADD COLUMN "color" VARCHAR(255)`,
	); err != nil {
		return fmt.Errorf("restoring objectives.color: %w", err)
	}
	return nil
}
