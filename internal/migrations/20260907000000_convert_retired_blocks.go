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

// m20260907000000_up exists because an unregistered type is silently skipped,
// will not import back, and cannot be removed in the editor.
//
// A task holds words someone typed, so it becomes a checklist of one: still a
// thing to tick off, and still a gate in the proof context it sat in. The
// location blocks hold "{}" and are deleted.
func m20260907000000_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "blocks") {
		return nil
	}

	if err := convertTaskBlocks(ctx, db); err != nil {
		return err
	}

	// They accepted nothing, so no state or uploads hang off them.
	if _, err := db.ExecContext(
		ctx, `DELETE FROM "blocks" WHERE "type" IN ('location_list', 'location_slot')`,
	); err != nil {
		return fmt.Errorf("deleting retired navigation blocks: %w", err)
	}

	return nil
}

// convertTaskBlocks rewrites in place so the page reads as it did.
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
		// An empty task still converts, so the author finds something to edit.
		_ = json.Unmarshal([]byte(row.Data), &task)

		itemID := uuid.New().String()
		data, err := json.Marshal(map[string]any{
			"content": "",
			"items": []map[string]any{{
				"id":          itemID,
				"description": task.Task,
				"checked":     false,
			}},
		})
		if err != nil {
			return fmt.Errorf("building checklist data for block %q: %w", row.ID, err)
		}

		// Stored, not derived, and duplication copies it.
		if _, err := db.ExecContext(ctx,
			`UPDATE "blocks" SET "type" = 'checklist', "data" = ?, "validation_required" = TRUE WHERE "id" = ?`,
			string(data), row.ID,
		); err != nil {
			return fmt.Errorf("converting task block %q: %w", row.ID, err)
		}

		// Or a finished team sees an unticked checklist marked complete.
		ticked, err := json.Marshal(map[string]any{"checked_items": []string{itemID}})
		if err != nil {
			return fmt.Errorf("building checklist state for block %q: %w", row.ID, err)
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE "run_block_states" SET "player_data" = ? WHERE "block_id" = ? AND "is_complete"`,
			string(ticked), row.ID,
		); err != nil {
			return fmt.Errorf("ticking completed task states for block %q: %w", row.ID, err)
		}
	}

	return nil
}

// m20260907000000_down is a no-op: authors may have edited the checklists.
func m20260907000000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
