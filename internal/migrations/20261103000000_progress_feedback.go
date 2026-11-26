package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261103000000_up, m20261103000000_down)
}

// Fraction is what existing quests showed, so running games do not change.
func m20261103000000_up(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "quest_settings", "progress_feedback") {
		return nil
	}
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE "quest_settings" ADD COLUMN "progress_feedback" VARCHAR(16) NOT NULL DEFAULT 'fraction'`); err != nil {
		return fmt.Errorf("adding quest_settings.progress_feedback: %w", err)
	}
	return nil
}

func m20261103000000_down(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quest_settings", "progress_feedback") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE "quest_settings" DROP COLUMN "progress_feedback"`); err != nil {
		return fmt.Errorf("dropping quest_settings.progress_feedback: %w", err)
	}
	return nil
}
