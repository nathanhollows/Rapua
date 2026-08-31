package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260831000000_up, m20260831000000_down)
}

// m20260831000000_objective is a point-in-time stand-in for models.Objective,
// which no longer knows about when_clause.
type m20260831000000_objective struct {
	bun.BaseModel `bun:"table:objectives"`

	ID         string `bun:"id,pk"`
	WhenClause string `bun:"when_clause"`
	Depends    string `bun:"depends"`
	ProofSets  string `bun:"proof_sets"`
	RevealSets string `bun:"reveal_sets"`
}

// m20260831000000_block mirrors models.Block for the data-column rewrite below.
type m20260831000000_block struct {
	bun.BaseModel `bun:"table:blocks"`

	ID   string `bun:"id,pk"`
	Data string `bun:"data"`
}

// m20260831000000_condition is the retired WhenClause condition shape, kept here
// so the conversion below can read what the old column holds.
type m20260831000000_condition struct {
	Var string `json:"var"`
	Op  string `json:"op,omitempty"`
	Not bool   `json:"not,omitempty"`
}

type m20260831000000_whenClause struct {
	AllOf []m20260831000000_condition `json:"all_of,omitempty"`
	AnyOf []m20260831000000_condition `json:"any_of,omitempty"`
}

// m20260831000000_up moves the format off when clauses and off map-shaped sets.
//
// Conditions are now a truthy-only list of variable names (`depends`), and
// `sets` is a list of names rather than a name-to-value map. Both shapes are
// already written to the database: 20260827000000 copies locations.when_clause
// into objectives.when_clause, and imported games carry map-shaped sets in
// objectives.proof_sets/reveal_sets and inside blocks.data. Left alone, the new
// code cannot unmarshal any of it, so this converts what converts and drops
// what does not.
//
// A when clause maps onto depends only when every condition is a bare truthy
// check ANDed together, which is what the new grammar can express. any_of is
// an OR and op is a comparison; neither survives, and such a clause is dropped
// rather than silently reinterpreted as something stricter or looser than the
// author wrote. Backwards compatibility is waived for the v8 format, so nothing
// is preserved beyond this.
//
// Group-level when clauses inside the quests.game_structure blob are left where
// they are. GameStructure no longer declares the field, so decoding ignores the
// key and export cannot re-emit it: it is inert rather than unreadable, and the
// column itself is going away with the blob.
func m20260831000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "depends") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" ADD COLUMN "depends" TEXT`); err != nil {
			return fmt.Errorf("adding objectives.depends: %w", err)
		}
	}

	if err := m20260831000000_convertObjectives(ctx, db); err != nil {
		return err
	}
	if err := m20260831000000_convertBlocks(ctx, db); err != nil {
		return err
	}

	if columnExists(ctx, db, "objectives", "when_clause") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "when_clause"`); err != nil {
			return fmt.Errorf("dropping objectives.when_clause: %w", err)
		}
	}

	return nil
}

// m20260831000000_convertObjectives rewrites when_clause into depends and both
// sets columns into their list form.
func m20260831000000_convertObjectives(ctx context.Context, db *bun.DB) error {
	hasWhen := columnExists(ctx, db, "objectives", "when_clause")

	cols := []string{"id", "depends", "proof_sets", "reveal_sets"}
	if hasWhen {
		cols = append(cols, "when_clause")
	}

	var objectives []m20260831000000_objective
	if err := db.NewSelect().Model(&objectives).Column(cols...).Scan(ctx); err != nil {
		return fmt.Errorf("loading objectives: %w", err)
	}

	for i := range objectives {
		obj := &objectives[i]

		depends := obj.Depends
		if hasWhen && depends == "" {
			converted, err := m20260831000000_whenToDepends(obj.WhenClause)
			if err != nil {
				return fmt.Errorf("converting objective %s when_clause: %w", obj.ID, err)
			}
			depends = converted
		}

		proof, err := m20260831000000_setsToList(obj.ProofSets)
		if err != nil {
			return fmt.Errorf("converting objective %s proof_sets: %w", obj.ID, err)
		}
		reveal, err := m20260831000000_setsToList(obj.RevealSets)
		if err != nil {
			return fmt.Errorf("converting objective %s reveal_sets: %w", obj.ID, err)
		}

		if depends == obj.Depends && proof == obj.ProofSets && reveal == obj.RevealSets {
			continue
		}

		obj.Depends, obj.ProofSets, obj.RevealSets = depends, proof, reveal
		if _, err := db.NewUpdate().Model(obj).
			Column("depends", "proof_sets", "reveal_sets").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("updating objective %s: %w", obj.ID, err)
		}
	}

	return nil
}

// m20260831000000_convertBlocks rewrites each block's stored data: map-shaped
// "sets" becomes a list, and "when" is removed outright. Export copies this
// column key-for-key into the document, so a leftover map-shaped "sets" would
// come back as a SETS_NOT_LIST error the next time the export is imported.
func m20260831000000_convertBlocks(ctx context.Context, db *bun.DB) error {
	var blocks []m20260831000000_block
	if err := db.NewSelect().Model(&blocks).Column("id", "data").Scan(ctx); err != nil {
		return fmt.Errorf("loading blocks: %w", err)
	}

	for i := range blocks {
		block := &blocks[i]
		if block.Data == "" {
			continue
		}

		var fields map[string]any
		if err := json.Unmarshal([]byte(block.Data), &fields); err != nil {
			// Not an object: nothing here holds a sets or when key.
			continue
		}

		_, hadWhen := fields["when"]
		delete(fields, "when")

		names, converted := m20260831000000_namesFromMap(fields["sets"])
		if converted {
			fields["sets"] = names
		}
		if !hadWhen && !converted {
			continue
		}

		data, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("re-encoding block %s data: %w", block.ID, err)
		}
		block.Data = string(data)
		if _, err := db.NewUpdate().Model(block).Column("data").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("updating block %s: %w", block.ID, err)
		}
	}

	return nil
}

// m20260831000000_whenToDepends renders a stored when clause as a depends list,
// or as an empty string when the clause cannot be expressed as one.
func m20260831000000_whenToDepends(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}

	var clause m20260831000000_whenClause
	if err := json.Unmarshal([]byte(raw), &clause); err != nil {
		// A clause the old code could not read either: drop it.
		return "", nil //nolint:nilerr // unreadable stored JSON is dropped, not fatal
	}
	// any_of is an OR, which depends cannot express at all.
	if len(clause.AnyOf) > 0 || len(clause.AllOf) == 0 {
		return "", nil
	}

	names := make([]string, 0, len(clause.AllOf))
	for _, cond := range clause.AllOf {
		// A comparison has no truthy-only equivalent.
		if cond.Op != "" || cond.Var == "" {
			return "", nil
		}
		if cond.Not {
			names = append(names, "not "+cond.Var)
			continue
		}
		names = append(names, cond.Var)
	}

	data, err := json.Marshal(names)
	if err != nil {
		return "", fmt.Errorf("encoding depends: %w", err)
	}
	return string(data), nil
}

// m20260831000000_setsToList rewrites a stored {"name": "value"} sets column as
// a list of names, leaving an already-converted list untouched.
func m20260831000000_setsToList(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return raw, nil //nolint:nilerr // unreadable stored JSON is left as-is, not fatal
	}

	names, converted := m20260831000000_namesFromMap(decoded)
	if !converted {
		return raw, nil
	}

	data, err := json.Marshal(names)
	if err != nil {
		return "", fmt.Errorf("encoding sets: %w", err)
	}
	return string(data), nil
}

// m20260831000000_namesFromMap returns the sorted keys of a map-shaped sets
// value. converted is false for any other shape, including an already-converted
// list, so callers can leave those alone. Keys are sorted only so a rerun
// produces byte-identical output.
func m20260831000000_namesFromMap(v any) ([]string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	names := make([]string, 0, len(m))
	for name := range m {
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, true
}

// m20260831000000_down restores the column pair. Schema only: the when clauses
// and map-shaped sets this migration rewrote are not reconstructed.
func m20260831000000_down(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "when_clause") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" ADD COLUMN "when_clause" TEXT`); err != nil {
			return fmt.Errorf("restoring objectives.when_clause: %w", err)
		}
	}

	if columnExists(ctx, db, "objectives", "depends") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "objectives" DROP COLUMN "depends"`); err != nil {
			return fmt.Errorf("dropping objectives.depends: %w", err)
		}
	}

	return nil
}
