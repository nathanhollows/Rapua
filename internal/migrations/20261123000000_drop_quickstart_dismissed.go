package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261123000000_up, m20261123000000_down)
}

// m20261123000000_up drops the flag for a bar that no longer exists.
func m20261123000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quests", "is_quick_start_dismissed") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE "quests" DROP COLUMN "is_quick_start_dismissed"`); err != nil {
		return fmt.Errorf("dropping quests.is_quick_start_dismissed: %w", err)
	}
	return nil
}

func m20261123000000_down(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "quests", "is_quick_start_dismissed") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE "quests" ADD COLUMN "is_quick_start_dismissed" BOOL`); err != nil {
		return fmt.Errorf("adding quests.is_quick_start_dismissed: %w", err)
	}
	return nil
}
