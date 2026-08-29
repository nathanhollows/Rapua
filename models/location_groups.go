package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/game"
)

// CompletionType is a type alias; defined in game/ and re-exported here.
type CompletionType = game.CompletionType

const (
	CompletionAll     = game.CompletionAll
	CompletionMinimum = game.CompletionMinimum
)

// GameStructure represents the hierarchical game state structure
//
// IMPORTANT ARCHITECTURE NOTES:
//
// Order Invariant:
//   - ObjectiveIDs are ALWAYS stored before SubGroups
//   - Array order is preserved on save/load
//   - No explicit ordering fields needed - position in array = order
//
// Root Group Behavior:
//   - Every instance has exactly ONE root group (IsRoot: true)
//   - The root group is NEVER rendered in the UI
//   - The root group acts as an invisible container for:
//   - Visible subgroups (rendered as group cards)
//   - Ungrouped objectives (rendered directly in the root area)
//   - Root group Name is always empty ("")
//   - Root group Color is always empty ("")
//
// Visible Groups:
//   - All visible groups in the UI are SubGroups of the root
//   - Visible groups have IsRoot: false
//   - Visible groups always have a Name and Color
//   - Visible groups are rendered as collapsible group cards
//
// Example Structure:
//
//	Root (IsRoot: true, Name: "")
//	├── ObjectiveIDs: ["obj1", "obj2"]  // Rendered in root area
//	└── SubGroups:
//	    ├── Group "Museum Tour" (visible group card)
//	    │   └── ObjectiveIDs: ["obj3", "obj4"]
//	    └── Group "Historical Sites" (visible group card)
//	        └── ObjectiveIDs: ["obj5", "obj6"]
//
//nolint:recvcheck // Value() requires value receiver, Scan() requires pointer receiver per database/sql interface
type GameStructure struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`                       // Empty for root group, required for visible groups
	Color           string           `json:"color"`                      // Empty for root, required for visible groups (e.g., "primary", "secondary")
	Routing         RouteStrategy    `json:"routing"`                    // ordered, randomised, free_roam, secret
	CompletionType  CompletionType   `json:"completion_type"`            // all, minimum
	MinimumRequired int              `json:"minimum_required,omitempty"` // For minimum completion type
	MaxNext         int              `json:"max_next,omitempty"`         // Max objectives to show for random routing (0 = unlimited).
	AutoAdvance     bool             `json:"auto_advance"`               // If true, auto-move to next group when CompletionType met
	IsRoot          bool             `json:"is_root"`                    // true ONLY for the invisible root container
	When            *game.WhenClause `json:"when,omitempty"`             // Visibility condition; nil = always visible. No migration needed: stored inside the JSON blob in instances.game_structure.

	// Storage: objectives first, then subgroups - order preserved in arrays.
	ObjectiveIDs []string        `json:"objective_ids,omitempty"` // Ordered list of objective IDs.
	SubGroups    []GameStructure `json:"sub_groups"`              // Ordered list of nested groups.

	// Runtime fields - populated by GameStructureService
	Objectives []*Objective `json:"-"`
	populated  bool         `json:"-"` // Set when GameStructureService has loaded Objectives.
}

// Clone returns a deep copy of gs, safe to mutate (e.g. remapping IDs) without
// affecting the original: a plain struct assignment shares the ObjectiveIDs
// and SubGroups backing arrays, so in-place edits to the copy would otherwise
// leak back into the source.
func (gs GameStructure) Clone() GameStructure {
	clone := gs
	clone.ObjectiveIDs = append([]string(nil), gs.ObjectiveIDs...)
	if gs.SubGroups != nil {
		clone.SubGroups = make([]GameStructure, len(gs.SubGroups))
		for i, sub := range gs.SubGroups {
			clone.SubGroups[i] = sub.Clone()
		}
	}
	return clone
}

// Scan implements the sql.Scanner interface for database unmarshalling.
func (gs *GameStructure) Scan(value any) error {
	if value == nil {
		*gs = GameStructure{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into GameStructure", value)
	}

	if len(data) == 0 {
		*gs = GameStructure{}
		return nil
	}

	err := json.Unmarshal(data, gs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal GameStructure: %w", err)
	}

	// Initialize slices to avoid nil pointer issues
	if gs.ObjectiveIDs == nil {
		gs.ObjectiveIDs = []string{}
	}
	if gs.SubGroups == nil {
		gs.SubGroups = []GameStructure{}
	}
	if gs.Objectives == nil {
		gs.Objectives = []*Objective{}
	}

	return nil
}

// Value implements the driver.Valuer interface for database marshalling.
func (gs GameStructure) Value() (driver.Value, error) {
	if gs.ID == "" {
		return nil, nil //nolint:nilnil // nil,nil is idiomatic for driver.Valuer to represent SQL NULL
	}

	data, err := json.Marshal(gs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GameStructure: %w", err)
	}

	return string(data), nil
}

// RemoveObjectiveID reports whether anything was removed, so callers can skip
// a needless write.
func (gs *GameStructure) RemoveObjectiveID(objectiveID string) bool {
	removed := false

	kept := make([]string, 0, len(gs.ObjectiveIDs))
	for _, id := range gs.ObjectiveIDs {
		if id == objectiveID {
			removed = true
			continue
		}
		kept = append(kept, id)
	}
	gs.ObjectiveIDs = kept

	for i := range gs.SubGroups {
		if gs.SubGroups[i].RemoveObjectiveID(objectiveID) {
			removed = true
		}
	}

	return removed
}

func (gs *GameStructure) IsPopulated() bool {
	return gs.populated
}

func (gs *GameStructure) SetPopulated(populated bool) {
	gs.populated = populated
}
