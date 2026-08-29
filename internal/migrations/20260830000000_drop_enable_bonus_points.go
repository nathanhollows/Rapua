package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260830000000_up, m20260830000000_down)
}

// m20260830000000_up drops quest_settings.enable_bonus_points: models.QuestSettings
// has had no such field since the bonus-points system was removed, but nothing
// ever dropped the column itself, and 20260329000000_cascade_deletes.go's table
// recreations kept carrying it forward unused.
func m20260830000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quest_settings", "enable_bonus_points") {
		return nil
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE "quest_settings" DROP COLUMN "enable_bonus_points"`); err != nil {
		return fmt.Errorf("dropping quest_settings.enable_bonus_points: %w", err)
	}

	return nil
}

// m20260830000000_down restores the column (schema only, not data).
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
