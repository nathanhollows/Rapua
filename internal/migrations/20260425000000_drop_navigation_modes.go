package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/uptrace/bun"
)

// Timestamped snapshot of the GameStructure JSON shape as of this migration.
// The Navigation field is json.RawMessage because it may be stored as int (0–4)
// from the 20251030094613 migration, or as a string ("custom", "tasks", etc.)
// from later code.
type m20260425_GameStructure struct {
	ID          string                    `json:"id"`
	LocationIDs []string                  `json:"location_ids"`
	SubGroups   []m20260425_GameStructure `json:"sub_groups"`
	Navigation  json.RawMessage           `json:"navigation"`
}

type m20260425_instance struct {
	bun.BaseModel `bun:"table:instances"`

	ID            string `bun:"id,pk,type:varchar(36)"`
	GameStructure string `bun:"game_structure,type:text"`
}

// parseNavMode extracts the navigation mode string from a raw JSON value.
// Handles both integer (legacy) and string (current) storage formats.
func m20260425_parseNavMode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try integer (legacy format from 20251030094613)
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		switch n {
		case 0:
			return "map"
		case 1:
			return "labelled_map"
		case 2:
			return "location_list"
		case 3:
			// NavigationDisplayClues → treat same as custom (uses location_clues context)
			return "custom"
		case 4:
			return "custom"
		case 5:
			return "tasks"
		default:
			return strconv.Itoa(n)
		}
	}

	return ""
}

// walkGroups recursively walks the group tree and builds a map of locationID → navMode.
// Child groups inherit parent navMode only if they have no explicit navigation set.
func m20260425_walkGroups(group m20260425_GameStructure, parentMode string) map[string]string {
	result := make(map[string]string)

	mode := m20260425_parseNavMode(group.Navigation)
	if mode == "" {
		mode = parentMode
	}

	for _, locID := range group.LocationIDs {
		result[locID] = mode
	}

	for _, sub := range group.SubGroups {
		for k, v := range m20260425_walkGroups(sub, mode) {
			result[k] = v
		}
	}

	return result
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Load all instances with their game_structure JSON.
			var instances []m20260425_instance
			if err := db.NewSelect().Model(&instances).Scan(ctx); err != nil {
				return fmt.Errorf("20260425_drop_navigation_modes: fetch instances: %w", err)
			}

			for _, instance := range instances {
				if instance.GameStructure == "" {
					continue
				}

				var root m20260425_GameStructure
				if err := json.Unmarshal([]byte(instance.GameStructure), &root); err != nil {
					return fmt.Errorf(
						"20260425_drop_navigation_modes: parse game_structure for instance %s: %w",
						instance.ID,
						err,
					)
				}

				// Build locationID → navMode map from root's sub-groups.
				locationModes := make(map[string]string)
				rootMode := m20260425_parseNavMode(root.Navigation)
				for _, sub := range root.SubGroups {
					for k, v := range m20260425_walkGroups(sub, rootMode) {
						locationModes[k] = v
					}
				}
				// Also handle locations directly on root.
				for _, locID := range root.LocationIDs {
					if _, exists := locationModes[locID]; !exists {
						locationModes[locID] = rootMode
					}
				}

				for locID, mode := range locationModes {
					switch mode {
					case "custom", "":
						// Keep location_clues → rename to navigation; delete task blocks.
						if _, err := db.ExecContext(
							ctx,
							`UPDATE blocks SET context = 'navigation' WHERE owner_id = ? AND context = 'location_clues'`,
							locID,
						); err != nil {
							return fmt.Errorf(
								"20260425_drop_navigation_modes: update location_clues for location %s: %w",
								locID,
								err,
							)
						}
						if _, err := db.ExecContext(ctx,
							`DELETE FROM blocks WHERE owner_id = ? AND context = 'task'`,
							locID,
						); err != nil {
							return fmt.Errorf(
								"20260425_drop_navigation_modes: delete task blocks for location %s: %w",
								locID,
								err,
							)
						}

					case "tasks":
						// Keep task → rename to navigation; delete location_clues blocks.
						if _, err := db.ExecContext(ctx,
							`UPDATE blocks SET context = 'navigation' WHERE owner_id = ? AND context = 'task'`,
							locID,
						); err != nil {
							return fmt.Errorf(
								"20260425_drop_navigation_modes: update task blocks for location %s: %w",
								locID,
								err,
							)
						}
						if _, err := db.ExecContext(ctx,
							`DELETE FROM blocks WHERE owner_id = ? AND context = 'location_clues'`,
							locID,
						); err != nil {
							return fmt.Errorf(
								"20260425_drop_navigation_modes: delete location_clues for location %s: %w",
								locID,
								err,
							)
						}

					default:
						// map, labelled_map, location_list — blocks in these contexts were never rendered.
						if _, err := db.ExecContext(ctx,
							`DELETE FROM blocks WHERE owner_id = ? AND context IN ('location_clues', 'task')`,
							locID,
						); err != nil {
							return fmt.Errorf(
								"20260425_drop_navigation_modes: delete nav blocks for location %s: %w",
								locID,
								err,
							)
						}
					}
				}
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Down migration: rename 'navigation' context blocks back to 'location_clues'.
			// We cannot recover deleted blocks or know the original mode, so this is best-effort.
			_, err := db.ExecContext(ctx,
				`UPDATE blocks SET context = 'location_clues' WHERE context = 'navigation'`)
			return err
		},
	)
}
