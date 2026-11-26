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

// removedBlockOptions lists, per block type, the option keys v8 dropped. Add to
// it rather than writing another migration while v8 is unreleased.
var removedBlockOptions = map[string][]string{
	"image":       {"full_width"},
	"toggle_text": {"small"},
	"rating":      {"max_rating"},
}

// itemLists names, per block type, where its items live and what to call one that
// has no id. A quiz names an option by its place, as the editor does.
var itemLists = map[string]struct{ key, prefix string }{
	"quiz":      {key: "options", prefix: "option_"},
	"sorting":   {key: "items", prefix: "item_"},
	"checklist": {key: "items", prefix: "item_"},
}

// m20260907000000_up exists because an unregistered type is silently skipped,
// will not import back, and cannot be removed in the editor.
//
// A task holds words someone typed, so it becomes a checklist of one: still a
// thing to tick off, and still a gate in the proof context it sat in. The
// location blocks hold "{}" and are deleted.
//
// It then tidies what the blocks that remain stored before v8. Options v8 dropped
// are cleared, because export writes a block's stored data as it is, so a leftover
// key would come out of every export and draw an unknown-field warning on the way
// back in. And every item that lacks an id is given one: items told apart by id
// collapse into one when they share an empty id, so every sorting row reads as the
// last and every quiz option counts as picked. Importing a game written without ids
// used to store them that way. It runs last so a converted checklist is not renamed.
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

	for blockType, keys := range removedBlockOptions {
		if err := dropBlockOptions(ctx, db, blockType, keys); err != nil {
			return err
		}
	}
	for blockType, list := range itemLists {
		if err := assignItemIDs(ctx, db, blockType, list.key, list.prefix); err != nil {
			return err
		}
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

func dropBlockOptions(ctx context.Context, db *bun.DB, blockType string, keys []string) error {
	var rows []struct {
		ID   string `bun:"id"`
		Data string `bun:"data"`
	}
	if err := db.NewRaw(
		`SELECT "id", "data" FROM "blocks" WHERE "type" = ?`, blockType,
	).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("loading %s blocks: %w", blockType, err)
	}

	for _, row := range rows {
		// Unreadable data is left as found: the key cannot be told apart from
		// the rest, and a migration is no place to guess.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row.Data), &fields); err != nil {
			continue
		}
		found := false
		for _, key := range keys {
			if _, ok := fields[key]; ok {
				delete(fields, key)
				found = true
			}
		}
		if !found {
			continue
		}
		rewritten, err := json.Marshal(fields)
		if err != nil {
			continue
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE "blocks" SET "data" = ? WHERE "id" = ?`, string(rewritten), row.ID,
		); err != nil {
			return fmt.Errorf("clearing options on block %q: %w", row.ID, err)
		}
	}
	return nil
}

func assignItemIDs(ctx context.Context, db *bun.DB, blockType, key, prefix string) error {
	var rows []struct {
		ID   string `bun:"id"`
		Data string `bun:"data"`
	}
	if err := db.NewRaw(`SELECT "id", "data" FROM "blocks" WHERE "type" = ?`, blockType).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("loading %s blocks: %w", blockType, err)
	}

	for _, row := range rows {
		// Unreadable data is left as found: a migration is no place to guess.
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(row.Data), &fields) != nil {
			continue
		}
		var items []map[string]json.RawMessage
		if json.Unmarshal(fields[key], &items) != nil {
			continue
		}
		if !fillMissingIDs(items, prefix) {
			continue
		}
		rewritten, err := json.Marshal(items)
		if err != nil {
			continue
		}
		fields[key] = rewritten
		data, err := json.Marshal(fields)
		if err != nil {
			continue
		}
		if _, err := db.ExecContext(
			ctx,
			`UPDATE "blocks" SET "data" = ? WHERE "id" = ?`,
			string(data),
			row.ID,
		); err != nil {
			return fmt.Errorf("naming items of block %q: %w", row.ID, err)
		}
	}
	return nil
}

// fillMissingIDs names each item without an id by its place, skipping any name an
// item already carries. It reports whether it changed anything.
func fillMissingIDs(items []map[string]json.RawMessage, prefix string) bool {
	taken := map[string]bool{}
	for _, item := range items {
		var id string
		if json.Unmarshal(item["id"], &id) == nil && id != "" {
			taken[id] = true
		}
	}

	changed := false
	for i, item := range items {
		var id string
		if json.Unmarshal(item["id"], &id) == nil && id != "" {
			continue
		}
		name := fmt.Sprintf("%s%d", prefix, i)
		for suffix := 1; taken[name]; suffix++ {
			name = fmt.Sprintf("%s%d_%d", prefix, i, suffix)
		}
		taken[name] = true
		quoted, _ := json.Marshal(name)
		item["id"] = quoted
		changed = true
	}
	return changed
}

// m20260907000000_down is a no-op: authors may have edited the checklists, the
// options are gone from the code so a restored value would have nowhere to be read,
// and an id carries no meaning to take back.
func m20260907000000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
