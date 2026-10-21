package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20261020000000_up, m20261020000000_down)
}

type m20261020000000_block struct {
	bun.BaseModel `bun:"table:blocks"`

	ID   string `bun:"id,pk"`
	Type string `bun:"type"`
	Data string `bun:"data"`
}

// m20261020000000_setters are the block types that may set a variable, frozen
// as they stood when this migration was written. A choice is left out because
// its options set their own; team_name because it completes outside check-in.
var m20261020000000_setters = map[string]bool{
	"checklist": true, "clue": true, "free_text": true, "geofence": true, "password": true,
	"photo": true, "pincode": true, "quiz": true, "rating": true, "scan": true, "sorting": true,
}

// m20261020000000_up rewrites each block's "sets" as the single name a block now
// carries. 20260831000000 normalised it to a list and nothing removed it when
// variables went, so stored data still holds lists that no longer parse.
//
// A list keeps its first name. Nothing could wait on a variable between their
// removal and their return, so no gate depends on the names dropped. A block
// type that may not set a variable loses the key, since it would only ever be
// reported as an error.
func m20261020000000_up(ctx context.Context, db *bun.DB) error {
	var rows []m20261020000000_block
	if err := db.NewSelect().Model(&rows).Column("id", "type", "data").Scan(ctx); err != nil {
		return fmt.Errorf("loading blocks: %w", err)
	}

	for i := range rows {
		row := &rows[i]
		var fields map[string]any
		if row.Data == "" || json.Unmarshal([]byte(row.Data), &fields) != nil {
			continue
		}
		raw, ok := fields["sets"]
		if !ok {
			continue
		}
		name := m20261020000000_firstName(raw)
		if name == "" || !m20261020000000_setters[row.Type] {
			delete(fields, "sets")
		} else {
			fields["sets"] = name
		}

		data, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("re-encoding block %s data: %w", row.ID, err)
		}
		row.Data = string(data)
		if _, err := db.NewUpdate().Model(row).Column("data").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("updating block %s: %w", row.ID, err)
		}
	}
	return nil
}

// m20261020000000_firstName reads a name from either shape "sets" has held.
func m20261020000000_firstName(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// m20261020000000_down does nothing: the code below this migration never reads
// a block's sets, so either shape is inert there.
func m20261020000000_down(context.Context, *bun.DB) error {
	return nil
}
