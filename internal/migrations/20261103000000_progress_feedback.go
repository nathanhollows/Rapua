package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261103000000_up, m20261103000000_down)
}

// m20261103000000_up lets an author choose what players are told about progress.
//
// Existing quests default to "fraction", what they already showed: anything else
// would change a running game under its players.
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

// m20261103000000_down drops the column. Every quest goes back to fraction, the
// only feedback the schema below this migration can express.
func m20261103000000_down(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quest_settings", "progress_feedback") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE "quest_settings" DROP COLUMN "progress_feedback"`); err != nil {
		return fmt.Errorf("dropping quest_settings.progress_feedback: %w", err)
	}
	return nil
}
