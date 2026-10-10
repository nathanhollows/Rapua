package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261010010000_up, m20261010010000_down)
}

// m20261010010000_up renames random_clue blocks to random_text.
//
// The block shows each team one text from a list, and its old name taught the
// retired clue-and-QR model to anyone who opened the block picker. The list key
// moves with the type so stored blocks match the field the spec now names and
// nothing has to translate between the two spellings.
func m20261010010000_up(ctx context.Context, db *bun.DB) error {
	return renameRandomText(ctx, db, "random_clue", "random_text", "clues", "texts")
}

// m20261010010000_down restores the old names. Reversible in full: the rename
// rewrites two strings and drops nothing.
func m20261010010000_down(ctx context.Context, db *bun.DB) error {
	return renameRandomText(ctx, db, "random_text", "random_clue", "texts", "clues")
}

// renameRandomText rewrites the type and the list key of every matching block
// in place, keeping each row's id, owner, context and ordering so the block
// stays where it was on the page. A row whose data does not parse still gets
// the new type: the registry only needs the type, and an unreadable list
// cannot be renamed anyway.
func renameRandomText(ctx context.Context, db *bun.DB, from, to, fromKey, toKey string) error {
	if !tableExists(ctx, db, "blocks") {
		return nil
	}

	var rows []struct {
		ID   string `bun:"id"`
		Data string `bun:"data"`
	}
	if err := db.NewRaw(
		`SELECT "id", "data" FROM "blocks" WHERE "type" = ?`, from,
	).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("loading %s blocks: %w", from, err)
	}

	for _, row := range rows {
		data := row.Data
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row.Data), &fields); err == nil {
			if list, ok := fields[fromKey]; ok {
				fields[toKey] = list
				delete(fields, fromKey)
				if rewritten, err := json.Marshal(fields); err == nil {
					data = string(rewritten)
				}
			}
		}

		if _, err := db.ExecContext(ctx,
			`UPDATE "blocks" SET "type" = ?, "data" = ? WHERE "id" = ?`, to, data, row.ID,
		); err != nil {
			return fmt.Errorf("renaming block %q: %w", row.ID, err)
		}
	}

	return nil
}
