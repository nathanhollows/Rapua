package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260907000000_up, m20260907000000_down)
}

// m20260907000000_up clears the block types that outlived their registration.
//
// An unregistered type fails three ways, and the quiet one is why this exists:
// the loader skips it, so the block is simply absent from the page with nothing
// said either way. Lint reports UNKNOWN_BLOCK_TYPE, and export writes the type
// out regardless of the registry, so the document will not import back.
//
// The editor offers no way to remove them either, since it renders blocks
// through the same registry that does not know them. Left alone they are rot
// no author can reach.
//
// Task blocks become a checklist of one item and the rest are deleted. The
// difference is whether a person wrote anything into the row: a task holds a
// sentence somebody typed, while location_list and location_slot hold "{}" and
// are structural residue from the navigation displays that were removed with
// Location. A checklist rather than plain text because a task was something a
// player did and ticked off, which a checklist of one still is, and because
// every task block sat in a proof context: a checklist is interactive, so
// those contexts gain the gate they have been missing rather than being left
// with nothing to satisfy.
func m20260907000000_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "blocks") {
		return nil
	}

	if err := convertTaskBlocks(ctx, db); err != nil {
		return err
	}

	// No player state or uploads have ever hung off these two: they render
	// nothing and accept nothing. Deleted directly rather than through the
	// broker migration's detach dance, which existed because broker blocks
	// took answers.
	if _, err := db.ExecContext(
		ctx, `DELETE FROM "blocks" WHERE "type" IN ('location_list', 'location_slot')`,
	); err != nil {
		return fmt.Errorf("deleting retired navigation blocks: %w", err)
	}

	return nil
}

// convertTaskBlocks rewrites each task block in place, keeping its id, owner,
// context and ordering so the objective's page reads as it did.
func convertTaskBlocks(ctx context.Context, db *bun.DB) error {
	var rows []struct {
		ID   string `bun:"id"`
		Data string `bun:"data"`
	}
	if err := db.NewRaw(
		`SELECT "id", "data" FROM "blocks" WHERE "type" = 'task'`,
	).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("loading task blocks: %w", err)
	}

	for _, row := range rows {
		var task struct {
			Task string `json:"task"`
		}
		// A task with no text converts to an empty checklist item rather than
		// being dropped: the block held a place on the page, and an author
		// looking for it should find something to edit.
		_ = json.Unmarshal([]byte(row.Data), &task)

		data, err := json.Marshal(map[string]any{
			"content": "",
			"items": []map[string]any{{
				"id":          uuid.New().String(),
				"description": task.Task,
				"checked":     false,
			}},
		})
		if err != nil {
			return fmt.Errorf("building checklist data for block %q: %w", row.ID, err)
		}

		if _, err := db.ExecContext(ctx,
			`UPDATE "blocks" SET "type" = 'checklist', "data" = ? WHERE "id" = ?`,
			string(data), row.ID,
		); err != nil {
			return fmt.Errorf("converting task block %q: %w", row.ID, err)
		}
	}

	return nil
}

// m20260907000000_down does nothing. A converted task is now a checklist an
// author may have edited, and the deleted blocks rendered nothing to restore.
func m20260907000000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
