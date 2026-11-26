package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260903000000_up, m20260903000000_down)
}

// m20260903000000_up drops game_structure so two records of the tree cannot
// disagree.
//
// Skips are dropped, not converted: a skip said a team could move on, not that
// the group was done, so a section_finishes row would fabricate a completion.
// A run holding one finds that group open again.
func m20260903000000_up(ctx context.Context, db *bun.DB) error {
	for _, col := range []struct{ table, name string }{
		{"quests", "game_structure"},
		{"runs", "skipped_group_ids"},
	} {
		if !columnExists(ctx, db, col.table, col.name) {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE %q DROP COLUMN %q`, col.table, col.name)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("dropping %s.%s: %w", col.table, col.name, err)
		}
	}
	return nil
}

// m20260903000000_down restores the columns empty: nothing derives the blob
// back, and the skipped group ids name groups 20260827000000 replaced. The '[]'
// default is there because the array scanner cannot read NULL.
func m20260903000000_down(ctx context.Context, db *bun.DB) error {
	for _, col := range []struct{ table, name, ddl string }{
		{"quests", "game_structure", "text"},
		{"runs", "skipped_group_ids", "text default '[]'"},
	} {
		if columnExists(ctx, db, col.table, col.name) {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE %q ADD COLUMN %q %s`, col.table, col.name, col.ddl)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("restoring %s.%s: %w", col.table, col.name, err)
		}
	}
	return nil
}
