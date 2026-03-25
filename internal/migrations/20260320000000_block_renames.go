package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/uptrace/bun"
)

// blockRenameTypeMap maps old block type strings to new ones.
var blockRenameTypeMap = map[string]string{
	"answer":            "password",
	"quiz_block":        "quiz",
	"markdown":          "text",
	"game_status_alert": "game_status",
	"start_game_button": "start_button",
}

// blockFieldRenames maps block types (new names) to their field renames.
// Each entry is old_key -> new_key.
var blockFieldRenames = map[string]map[string]string{
	"header": {
		"title_text": "title",
		"title_size": "size",
	},
	"quiz": {
		"retry_enabled":   "allow_retry",
		"randomize_order": "randomise_order",
		"retry":           "allow_retry",
		"randomize":       "randomise_order",
	},
	"image": {
		"content": "url",
	},
	"youtube": {
		"content": "url",
	},
	"checklist": {
		"list": "items",
	},
	"sorting": {
		"scoring_scheme": "scoring",
	},
	"clue": {
		"clue_text":        "clue",
		"description_text": "description",
	},
	"broker": {
		"information_tiers": "tiers",
	},
	"team_name": {
		"block_text": "prompt",
		"text":       "prompt",
	},
	"start_button": {
		"scheduled_button_text": "scheduled_text",
		"active_button_text":    "active_text",
		"button_style":          "style",
	},
	"alert": {
		"variant": "style",
	},
	"button": {
		"variant": "style",
	},
}

// Also need to rename is_correct -> correct inside quiz option objects
var quizOptionFieldRenames = map[string]string{
	"is_correct": "correct",
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Step 1: Rename block types
		for oldType, newType := range blockRenameTypeMap {
			_, err := db.ExecContext(ctx,
				`UPDATE blocks SET type = ? WHERE type = ?`,
				newType, oldType,
			)
			if err != nil {
				return fmt.Errorf("renaming block type %s to %s: %w", oldType, newType, err)
			}
		}

		// Step 2: Rename JSON fields in block data
		// We need to process all block types that have field renames
		for blockType, fieldMap := range blockFieldRenames {
			if err := renameBlockFields(ctx, db, blockType, fieldMap); err != nil {
				return fmt.Errorf("renaming fields for block type %s: %w", blockType, err)
			}
		}

		// Step 3: Rename is_correct -> correct inside quiz option arrays
		if err := renameQuizOptionFields(ctx, db); err != nil {
			return fmt.Errorf("renaming quiz option fields: %w", err)
		}

		// Step 4: Remove the unimplemented fuzzy field from password blocks
		if err := removeJSONField(ctx, db, "password", "fuzzy"); err != nil {
			return fmt.Errorf("removing fuzzy field from password blocks: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Reverse: rename quiz option fields back
		if err := reverseQuizOptionFields(ctx, db); err != nil {
			return fmt.Errorf("reversing quiz option fields: %w", err)
		}

		// Reverse: rename JSON fields back
		reverseFieldRenames := make(map[string]map[string]string)
		for blockType, fieldMap := range blockFieldRenames {
			reverseFieldRenames[blockType] = make(map[string]string)
			for oldKey, newKey := range fieldMap {
				reverseFieldRenames[blockType][newKey] = oldKey
			}
		}
		// Map block types back to old names for the reverse
		reverseTypeForFields := make(map[string]string)
		for oldType, newType := range blockRenameTypeMap {
			reverseTypeForFields[newType] = oldType
		}

		for blockType, fieldMap := range reverseFieldRenames {
			// Use old type name if this block was renamed
			oldBlockType := blockType
			if mapped, ok := reverseTypeForFields[blockType]; ok {
				oldBlockType = mapped
			}
			if err := renameBlockFields(ctx, db, oldBlockType, fieldMap); err != nil {
				return fmt.Errorf("reversing fields for block type %s: %w", blockType, err)
			}
		}

		// Reverse: rename block types back
		for oldType, newType := range blockRenameTypeMap {
			_, err := db.ExecContext(ctx,
				`UPDATE blocks SET type = ? WHERE type = ?`,
				oldType, newType,
			)
			if err != nil {
				return fmt.Errorf("reversing block type %s to %s: %w", newType, oldType, err)
			}
		}

		return nil
	})
}

// renameBlockFields reads each block of the given type, renames JSON keys in data, and writes back.
func renameBlockFields(ctx context.Context, db *bun.DB, blockType string, fieldMap map[string]string) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, data FROM blocks WHERE type = ?`,
		blockType,
	)
	if err != nil {
		return fmt.Errorf("querying blocks: %w", err)
	}
	defer rows.Close()

	type blockRow struct {
		id   string
		data string
	}

	var blocks []blockRow
	for rows.Next() {
		var b blockRow
		if err := rows.Scan(&b.id, &b.data); err != nil {
			return fmt.Errorf("scanning block row: %w", err)
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating block rows: %w", err)
	}

	for _, b := range blocks {
		newData, changed, err := renameJSONKeys(b.data, fieldMap)
		if err != nil {
			return fmt.Errorf("renaming keys in block %s: %w", b.id, err)
		}
		if !changed {
			continue
		}

		_, err = db.ExecContext(ctx,
			`UPDATE blocks SET data = ? WHERE id = ?`,
			newData, b.id,
		)
		if err != nil {
			return fmt.Errorf("updating block %s: %w", b.id, err)
		}
	}

	return nil
}

// renameJSONKeys renames top-level keys in a JSON object string.
func renameJSONKeys(data string, fieldMap map[string]string) (string, bool, error) {
	if data == "" || data == "null" {
		return data, false, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return data, false, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	changed := false
	for oldKey, newKey := range fieldMap {
		if val, exists := obj[oldKey]; exists {
			obj[newKey] = val
			delete(obj, oldKey)
			changed = true
		}
	}

	if !changed {
		return data, false, nil
	}

	newData, err := json.Marshal(obj)
	if err != nil {
		return data, false, fmt.Errorf("marshaling JSON: %w", err)
	}

	return string(newData), true, nil
}

// renameQuizOptionFields renames is_correct -> correct inside quiz block option arrays.
func renameQuizOptionFields(ctx context.Context, db *bun.DB) error {
	return processQuizOptions(ctx, db, "is_correct", "correct")
}

func reverseQuizOptionFields(ctx context.Context, db *bun.DB) error {
	return processQuizOptions(ctx, db, "correct", "is_correct")
}

func processQuizOptions(ctx context.Context, db *bun.DB, oldKey, newKey string) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, data FROM blocks WHERE type IN (?, ?)`,
		"quiz", "quiz_block",
	)
	if err != nil {
		return fmt.Errorf("querying quiz blocks: %w", err)
	}
	defer rows.Close()

	type blockRow struct {
		id   string
		data sql.NullString
	}

	var blocks []blockRow
	for rows.Next() {
		var b blockRow
		if err := rows.Scan(&b.id, &b.data); err != nil {
			return fmt.Errorf("scanning quiz block row: %w", err)
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating quiz block rows: %w", err)
	}

	for _, b := range blocks {
		if !b.data.Valid || b.data.String == "" || b.data.String == "null" {
			continue
		}

		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(b.data.String), &obj); err != nil {
			return fmt.Errorf("unmarshaling quiz block %s: %w", b.id, err)
		}

		optionsRaw, exists := obj["options"]
		if !exists {
			continue
		}

		var options []map[string]json.RawMessage
		if err := json.Unmarshal(optionsRaw, &options); err != nil {
			return fmt.Errorf("unmarshaling options for quiz block %s: %w", b.id, err)
		}

		changed := false
		for i, opt := range options {
			if val, ok := opt[oldKey]; ok {
				opt[newKey] = val
				delete(opt, oldKey)
				options[i] = opt
				changed = true
			}
		}

		if !changed {
			continue
		}

		newOptions, err := json.Marshal(options)
		if err != nil {
			return fmt.Errorf("marshaling quiz options for block %s: %w", b.id, err)
		}
		obj["options"] = newOptions

		newData, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("marshaling quiz block %s: %w", b.id, err)
		}

		_, err = db.ExecContext(ctx,
			`UPDATE blocks SET data = ? WHERE id = ?`,
			string(newData), b.id,
		)
		if err != nil {
			return fmt.Errorf("updating quiz block %s: %w", b.id, err)
		}
	}

	return nil
}

// removeJSONField deletes a key from the JSON data blob of all blocks of the given type.
func removeJSONField(ctx context.Context, db *bun.DB, blockType, field string) error {
	rows, err := db.QueryContext(ctx, `SELECT id, data FROM blocks WHERE type = ?`, blockType)
	if err != nil {
		return fmt.Errorf("querying blocks: %w", err)
	}
	defer rows.Close()

	type blockRow struct {
		id   string
		data string
	}

	var blocks []blockRow
	for rows.Next() {
		var b blockRow
		if err := rows.Scan(&b.id, &b.data); err != nil {
			return fmt.Errorf("scanning block row: %w", err)
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating block rows: %w", err)
	}

	for _, b := range blocks {
		if b.data == "" || b.data == "null" {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(b.data), &obj); err != nil {
			return fmt.Errorf("unmarshaling block %s: %w", b.id, err)
		}
		if _, exists := obj[field]; !exists {
			continue
		}
		delete(obj, field)
		newData, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("marshaling block %s: %w", b.id, err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE blocks SET data = ? WHERE id = ?`, string(newData), b.id); err != nil {
			return fmt.Errorf("updating block %s: %w", b.id, err)
		}
	}

	return nil
}
