package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260830000000_up, m20260830000000_down)
}

// m20260830000000_up drops a column the model lost with bonus points, which
// 20260329000000's table recreations kept carrying.
func m20260830000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quest_settings", "enable_bonus_points") {
		return nil
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE "quest_settings" DROP COLUMN "enable_bonus_points"`); err != nil {
		return fmt.Errorf("dropping quest_settings.enable_bonus_points: %w", err)
	}

	return nil
}

func m20260830000000_down(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "quest_settings", "enable_bonus_points") {
		return nil
	}

	if _, err := db.ExecContext(
		ctx, `ALTER TABLE "quest_settings" ADD COLUMN "enable_bonus_points" BOOLEAN DEFAULT FALSE`,
	); err != nil {
		return fmt.Errorf("restoring quest_settings.enable_bonus_points: %w", err)
	}

	return nil
}
