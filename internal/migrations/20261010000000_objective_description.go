package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261010000000_up, m20261010000000_down)
}

// m20261010000000_up adds the objective's description: the detail a player needs
// once they are standing in front of the thing.
//
// The title became the task rather than a place name, which left it doing two
// jobs: short enough to read in a list, and complete enough to act on. This is
// the second half. "Find the plan chest" is the title; "Level 2, past the
// model-making room" is this.
//
// Called description rather than context, which is what block contexts are
// named after in this codebase, and rather than instruction, which is what the
// title now is.
func m20261010000000_up(ctx context.Context, db *bun.DB) error {
	if columnExists(ctx, db, "objectives", "description") {
		return nil
	}
	if _, err := db.ExecContext(
		ctx, `ALTER TABLE "objectives" ADD COLUMN "description" TEXT NOT NULL DEFAULT ''`,
	); err != nil {
		return fmt.Errorf("adding objectives.description: %w", err)
	}
	return nil
}

// m20261010000000_down drops the column. Anything written into it goes with it:
// the field is new, so there is no earlier home to put it back into.
func m20261010000000_down(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "description") {
		return nil
	}
	if _, err := db.ExecContext(
		ctx, `ALTER TABLE "objectives" DROP COLUMN "description"`,
	); err != nil {
		return fmt.Errorf("dropping objectives.description: %w", err)
	}
	return nil
}
