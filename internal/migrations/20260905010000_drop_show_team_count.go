package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260905010000_up, m20260905010000_down)
}

// m20260905010000_up drops quest_settings.show_team_count: models.QuestSettings
// has had no such field since the visiting-teams display was removed, and the
// column would otherwise sit unmapped, inviting a later reader to wonder
// whether the display still exists.
func m20260905010000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quest_settings", "show_team_count") {
		return nil
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE "quest_settings" DROP COLUMN "show_team_count"`); err != nil {
		return fmt.Errorf("dropping quest_settings.show_team_count: %w", err)
	}

	return nil
}

// m20260905010000_down restores the column (schema only, not data).
func m20260905010000_down(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "quest_settings", "show_team_count") {
		return nil
	}

	if _, err := db.ExecContext(
		ctx, `ALTER TABLE "quest_settings" ADD COLUMN "show_team_count" BOOLEAN DEFAULT FALSE`,
	); err != nil {
		return fmt.Errorf("restoring quest_settings.show_team_count: %w", err)
	}

	return nil
}
