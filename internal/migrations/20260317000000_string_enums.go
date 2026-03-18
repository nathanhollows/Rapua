package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

//nolint:gochecknoinits // Migration init pattern required by bun migrate framework
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return m20260317_rewriteEnums(ctx, db, m20260317_migrateGroup)
	}, func(ctx context.Context, db *bun.DB) error {
		return m20260317_rewriteEnums(ctx, db, m20260317_reverseGroup)
	})
}

// m20260317_rewriteEnums loads all game_structure JSON, applies fn to each, and saves back.
func m20260317_rewriteEnums(
	ctx context.Context,
	db *bun.DB,
	fn func(map[string]any) bool,
) error {
	var rows []m20260317_Row
	if err := db.NewSelect().
		TableExpr("instances").
		Column("id", "game_structure").
		Where("game_structure IS NOT NULL AND game_structure != ''").
		Scan(ctx, &rows); err != nil {
		return fmt.Errorf("selecting instances: %w", err)
	}

	for _, row := range rows {
		var structure map[string]any
		if err := json.Unmarshal([]byte(row.GameStructure), &structure); err != nil {
			slog.Warn( //nolint:sloglint // no logger available in migrations
				"skipping instance with unparseable game_structure",
				"id", row.ID, "error", err,
			)
			continue
		}

		if !fn(structure) {
			continue
		}

		data, err := json.Marshal(structure)
		if err != nil {
			return fmt.Errorf("marshalling game_structure for %s: %w", row.ID, err)
		}

		if _, err = db.ExecContext(ctx,
			`UPDATE instances SET game_structure = ? WHERE id = ?`,
			string(data), row.ID); err != nil {
			return fmt.Errorf("updating game_structure for %s: %w", row.ID, err)
		}
	}

	return nil
}

type m20260317_Row struct {
	ID            string `bun:"id"`
	GameStructure string `bun:"game_structure"`
}

// Mapping tables for forward migration (int → string).
//
//nolint:gochecknoglobals,mnd // Migration-scoped lookup tables with historical iota indices
var (
	m20260317_routingForward = map[float64]string{
		0: "randomised",
		1: "free_roam",
		2: "ordered",
		3: "secret",
	}

	m20260317_navigationForward = map[float64]string{
		0: "map",
		1: "labelled_map",
		2: "location_list",
		3: "custom", // deprecated NavigationDisplayClues
		4: "custom",
		5: "tasks",
	}

	m20260317_routingReverse = map[string]float64{
		"randomised": 0,
		"free_roam":  1,
		"ordered":    2,
		"secret":     3,
	}

	m20260317_navigationReverse = map[string]float64{
		"map":           0,
		"labelled_map":  1,
		"location_list": 2,
		"custom":        4,
		"tasks":         5,
	}
)

// m20260317_migrateGroup converts integer routing/navigation to strings recursively.
func m20260317_migrateGroup(group map[string]any) bool {
	changed := m20260317_convertField(group, "routing", m20260317_routingForward)
	if m20260317_convertField(group, "navigation", m20260317_navigationForward) {
		changed = true
	}

	subGroups, _ := group["sub_groups"].([]any)
	for _, sg := range subGroups {
		if sgMap, ok := sg.(map[string]any); ok && m20260317_migrateGroup(sgMap) {
			changed = true
		}
	}

	return changed
}

// m20260317_reverseGroup converts string routing/navigation back to integers recursively.
func m20260317_reverseGroup(group map[string]any) bool {
	changed := m20260317_reverseField(group, "routing", m20260317_routingReverse)
	if m20260317_reverseField(group, "navigation", m20260317_navigationReverse) {
		changed = true
	}

	subGroups, _ := group["sub_groups"].([]any)
	for _, sg := range subGroups {
		if sgMap, ok := sg.(map[string]any); ok && m20260317_reverseGroup(sgMap) {
			changed = true
		}
	}

	return changed
}

// m20260317_convertField converts a single float64 field to its string equivalent.
func m20260317_convertField(
	group map[string]any,
	key string,
	lookup map[float64]string,
) bool {
	num, ok := group[key].(float64)
	if !ok {
		return false
	}

	str, found := lookup[num]
	if !found {
		slog.Warn( //nolint:sloglint // no logger in migrations
			"unexpected enum value", "key", key, "value", num,
		)
		return false
	}

	group[key] = str
	return true
}

// m20260317_reverseField converts a single string field back to its float64 equivalent.
func m20260317_reverseField(
	group map[string]any,
	key string,
	lookup map[string]float64,
) bool {
	str, ok := group[key].(string)
	if !ok {
		return false
	}

	num, found := lookup[str]
	if !found {
		return false
	}

	group[key] = num
	return true
}
