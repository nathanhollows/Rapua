package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260901000000_up, m20260901000000_down)
}

// m20260901000000_objectiveColumns are the tree and node columns an objective
// needs now that a section is an objective like any other, rather than a group
// in a separate blob.
var m20260901000000_objectiveColumns = []struct {
	name string
	ddl  string
}{
	{"parent_id", `VARCHAR(36)`},
	{"position", `INT NOT NULL DEFAULT 0`},
	{"color", `VARCHAR(255)`},
	{"routing", `VARCHAR(32)`},
	{"children_min", `INT`},
	{"children_max", `INT`},
	{"max_next", `INT NOT NULL DEFAULT 0`},
	{"finish_label", `VARCHAR(255)`},
}

// m20260901000000_up gives the objectives table room to hold the whole tree,
// and adds the table recording a player's choice to end a section.
//
// The columns are added empty. A quest's structure is recorded in the group
// blob on quests.game_structure, and copying it into these columns as well
// would leave two records of the same thing free to disagree.
//
// parent_id carries no foreign key to objectives(id): the rows it points at
// live in the same table, and SQLite can only add a self-referencing constraint
// by recreating the table. Deletion already cascades from quests, and the
// repository enforces what the constraint would have.
func m20260901000000_up(ctx context.Context, db *bun.DB) error {
	for _, col := range m20260901000000_objectiveColumns {
		if columnExists(ctx, db, "objectives", col.name) {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE "objectives" ADD COLUMN %q %s`, col.name, col.ddl)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("adding objectives.%s: %w", col.name, err)
		}
	}

	// "order" was never written by anything; position replaces it.
	if columnExists(ctx, db, "objectives", "order") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "order"`); err != nil {
			return fmt.Errorf("dropping objectives.order: %w", err)
		}
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "section_finishes" (
		"run_code" VARCHAR(36) NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"objective_id" VARCHAR(36) NOT NULL REFERENCES "objectives"("id") ON DELETE CASCADE,
		"finished_at" DATETIME NOT NULL DEFAULT current_timestamp,
		PRIMARY KEY ("run_code", "objective_id")
	)`); err != nil {
		return fmt.Errorf("creating section_finishes: %w", err)
	}

	// section_finishes needs no index of its own: run_code leads its primary
	// key, so a lookup by run already uses it.

	// Children are always read by parent, and no constraint backs that column.
	if _, err := db.ExecContext(
		ctx,
		`CREATE INDEX IF NOT EXISTS "idx_objectives_parent_id" ON "objectives" ("parent_id")`,
	); err != nil {
		return fmt.Errorf("indexing objectives.parent_id: %w", err)
	}

	return nil
}

// m20260901000000_down removes what the up added, restoring the column the up
// dropped. Schema only: any values written into the new columns go with them.
func m20260901000000_down(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS "idx_objectives_parent_id"`); err != nil {
		return fmt.Errorf("dropping objectives.parent_id index: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "section_finishes"`); err != nil {
		return fmt.Errorf("dropping section_finishes: %w", err)
	}

	if !columnExists(ctx, db, "objectives", "order") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" ADD COLUMN "order" INT`); err != nil {
			return fmt.Errorf("restoring objectives.order: %w", err)
		}
	}

	for _, col := range m20260901000000_objectiveColumns {
		if !columnExists(ctx, db, "objectives", col.name) {
			continue
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE "objectives" DROP COLUMN %q`, col.name)); err != nil {
			return fmt.Errorf("dropping objectives.%s: %w", col.name, err)
		}
	}

	return nil
}
