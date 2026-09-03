package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260905000000_up, m20260905000000_down)
}

// m20260905000000_up gives "no parent" one spelling.
//
// The backfill out of the group blob wrote parent_id as a literal empty string,
// because it inserted rows with raw SQL; everything writing through the model
// writes NULL, since the column is nullzero. Both read as the root, and
// FindRoot accepts either, so the split went unnoticed until a unique index
// over (parent_id, position) arrived: NULLs are distinct to SQLite and empty
// strings are not, so migrated roots competed with each other for positions
// across quests while new roots competed with nothing.
//
// That is why every migrated root holds a different position: 41 quests were
// renumbered 0 to 40 as though they were one sibling list. Normalising to NULL
// settles them all at 0 and makes the constraint mean one thing.
func m20260905000000_up(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE "objectives" SET "parent_id" = NULL WHERE "parent_id" = ''`); err != nil {
		return fmt.Errorf("normalising empty parent_id to NULL: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE "objectives" SET "position" = 0 WHERE "parent_id" IS NULL`); err != nil {
		return fmt.Errorf("resetting root positions: %w", err)
	}
	return nil
}

// m20260905000000_down does nothing. The empty strings it replaced carried no
// information NULL does not, and putting them back would reintroduce the
// collision this removes.
func m20260905000000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
