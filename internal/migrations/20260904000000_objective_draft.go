package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260904000000_up, m20260904000000_down)
}

// m20260904000000_up gives an objective a draft state, so a section can be
// taken out of play and put back without losing where it sat.
//
// Existing rows are published. Every quest in the database was authored when
// creating an objective was the same as publishing it, and defaulting the other
// way would empty every live quest on deploy.
//
// The unique index over (parent_id, position) comes with it. Positions were
// only ever distinct by convention, and a tie is broken by id, which under
// ordered routing silently rewrites the order a quest is played in. Drafting
// makes that worse by giving authors a reason to move things constantly, so the
// database refuses a collision rather than the callers remembering not to
// cause one.
func m20260904000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "draft") {
		if _, err := db.ExecContext(ctx,
			`ALTER TABLE "objectives" ADD COLUMN "draft" BOOLEAN NOT NULL DEFAULT FALSE`); err != nil {
			return fmt.Errorf("adding objectives.draft: %w", err)
		}
	}

	if err := m20260904000000_densifyPositions(ctx, db); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx,
		`CREATE UNIQUE INDEX IF NOT EXISTS "idx_objectives_parent_position"`+
			` ON "objectives" ("parent_id", "position")`); err != nil {
		return fmt.Errorf("indexing objectives (parent_id, position): %w", err)
	}
	return nil
}

// m20260904000000_densifyPositions renumbers every sibling list so the unique
// index can be created at all. Rows written before it existed are free to share
// a position, and the conversion out of the group blob had no reason to check.
//
// The order they already sort in is preserved, id included, so a quest keeps
// being played the way it was: this settles ties rather than reshuffling.
func m20260904000000_densifyPositions(ctx context.Context, db *bun.DB) error {
	rows, err := db.QueryContext(ctx,
		`SELECT "id", COALESCE("parent_id", '') FROM "objectives"`+
			` ORDER BY COALESCE("parent_id", ''), "position", "id"`)
	if err != nil {
		return fmt.Errorf("loading objectives to renumber: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type row struct{ id, parentID string }
	var ordered []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.parentID); err != nil {
			return fmt.Errorf("scanning objective: %w", err)
		}
		ordered = append(ordered, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading objectives to renumber: %w", err)
	}

	// Shifted clear of every position in use before being settled: the index
	// this prepares for is checked per statement, not at commit, so renumbering
	// in place would collide with the rows not yet moved.
	const offset = 1000000
	if _, err := db.ExecContext(ctx,
		`UPDATE "objectives" SET "position" = "position" + ?`, offset); err != nil {
		return fmt.Errorf("shifting objectives clear: %w", err)
	}

	positions := map[string]int{}
	for _, r := range ordered {
		if _, err := db.ExecContext(ctx,
			`UPDATE "objectives" SET "position" = ? WHERE "id" = ?`, positions[r.parentID], r.id); err != nil {
			return fmt.Errorf("renumbering objective %s: %w", r.id, err)
		}
		positions[r.parentID]++
	}
	return nil
}

// m20260904000000_down drops both. Drafted objectives come back published,
// which is the only state the schema below this migration can express.
func m20260904000000_down(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS "idx_objectives_parent_position"`); err != nil {
		return fmt.Errorf("dropping objectives (parent_id, position) index: %w", err)
	}
	if columnExists(ctx, db, "objectives", "draft") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "draft"`); err != nil {
			return fmt.Errorf("dropping objectives.draft: %w", err)
		}
	}
	return nil
}
