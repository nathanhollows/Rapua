package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260903000000_up, m20260903000000_down)
}

// m20260903000000_up drops what the group blob left behind.
//
// quests.game_structure held the tree; it lives in objectives.parent_id and
// position now, and leaving the column would leave two records of the same
// thing free to disagree.
//
// runs.skipped_group_ids recorded a facilitator skipping a group past its
// minimum. A player ending a section is recorded in section_finishes instead,
// and it is that objective's completion rather than a note to skip past it.
//
// The stored skips are dropped rather than converted, deliberately. A skip
// asserted that a team could move on, not that the group was done, so writing
// a section_finishes row for one would fabricate a completion the band may not
// support. Any run holding a skip when this deploys finds that group open
// again.
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

// m20260903000000_down restores the columns empty, matching the DDL that
// created them (20251030094613): plain text, and for skipped_group_ids the
// '[]' default the array scanner needs, since a restored NULL is not a list.
//
// Empty is all it can restore. game_structure was the record the tree is now
// derived from and nothing derives it back, and the group ids in
// skipped_group_ids name blob groups that 20260902000000 replaced with freshly
// minted section rows, so there is no id left to map them onto. Rolling back
// loses both, knowingly.
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
