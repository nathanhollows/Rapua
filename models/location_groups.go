package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// CompletionType defines how a group is considered completed.
type CompletionType string

const (
	CompletionAll     CompletionType = "all"
	CompletionMinimum CompletionType = "minimum"
)

// GameStructure represents the hierarchical game state structure
//
// IMPORTANT ARCHITECTURE NOTES:
//
// Order Invariant:
//   - LocationIDs are ALWAYS stored before SubGroups
//   - Array order is preserved on save/load
//   - No explicit ordering fields needed - position in array = order
//
// Root Group Behavior:
//   - Every instance has exactly ONE root group (IsRoot: true)
//   - The root group is NEVER rendered in the UI
//   - The root group acts as an invisible container for:
//   - Visible subgroups (rendered as group cards)
//   - Ungrouped locations (rendered directly in the root locations area)
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
//	├── LocationIDs: ["loc1", "loc2"]  // Rendered in root locations-area
//	└── SubGroups:
//	    ├── Group "Museum Tour" (visible group card)
//	    │   └── LocationIDs: ["loc3", "loc4"]
//	    └── Group "Historical Sites" (visible group card)
//	        └── LocationIDs: ["loc5", "loc6"]
type GameStructure struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`                       // Empty for root group, required for visible groups
	Color           string                `json:"color"`                      // Empty for root, required for visible groups (e.g., "primary", "secondary")
	Routing         RouteStrategy         `json:"routing"`                    // ordered, random, free_roam
	Navigation      NavigationDisplayMode `json:"navigation"`                 // clues, map, map_names, names_only
	CompletionType  CompletionType        `json:"completion_type"`            // all, minimum
	MinimumRequired int                   `json:"minimum_required,omitempty"` // For minimum completion type
	MaxNext         int                   `json:"max_next,omitempty"`         // Max locations to show for random routing (0 = unlimited)
	AutoAdvance     bool                  `json:"auto_advance"`               // If true, auto-move to next group when CompletionType met
	IsRoot          bool                  `json:"is_root"`                    // true ONLY for the invisible root container

	// Storage: locations first, then subgroups - order preserved in arrays
	LocationIDs []string        `json:"location_ids"` // Ordered list of location IDs
	SubGroups   []GameStructure `json:"sub_groups"`   // Ordered list of nested groups

	// Runtime fields - populated by GameStructureService
	Locations []*Location `json:"-"` // Loaded location pointers
	populated bool        `json:"-"` // Private field to track if locations are loaded
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
	if gs.LocationIDs == nil {
		gs.LocationIDs = []string{}
	}
	if gs.SubGroups == nil {
		gs.SubGroups = []GameStructure{}
	}
	if gs.Locations == nil {
		gs.Locations = []*Location{}
	}

	return nil
}

// Value implements the driver.Valuer interface for database marshalling.
func (gs GameStructure) Value() (driver.Value, error) {
	if gs.ID == "" {
		return nil, nil
	}

	data, err := json.Marshal(gs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GameStructure: %w", err)
	}

	return string(data), nil
}

// IsPopulated returns whether this group has been populated with location data.
func (gs *GameStructure) IsPopulated() bool {
	return gs.populated
}

// SetPopulated marks this group as populated with location data.
func (gs *GameStructure) SetPopulated(populated bool) {
	gs.populated = populated
}

// GameContext wraps a GameStructure with its service for easy template usage
// This allows passing a single object that has both data and behavior.
type GameContext struct {
	Structure *GameStructure
	service   GameStructureServiceInterface // Interface for testing
}

// GameStructureServiceInterface defines the methods needed by GameContext.
type GameStructureServiceInterface interface {
	// Loading methods
	Load(ctx context.Context, instanceID string, group *GameStructure, recursive bool) error
	LoadByLocationID(ctx context.Context, instanceID string, locationID string) (*GameStructure, error)

	// Validation and persistence
	Validate(group *GameStructure, instanceID string) error
	Save(ctx context.Context, instanceID string, group *GameStructure) error

	// Navigation methods
	FindGroupByID(gameStructure *GameStructure, groupID string) *GameStructure
	GetAllLocationIDs(group *GameStructure) []string
	GetNextItemType(
		group *GameStructure,
		completedLocationIDs map[string]bool,
		completedGroupIDs map[string]bool,
	) interface{}
	GetNextLocation(group *GameStructure, completedLocationIDs map[string]bool, teamID string) string
	GetNextGroup(group *GameStructure, completedGroups map[string]bool) *GameStructure
	IsCompleted(group *GameStructure, completedCount int) bool
}

// NewGameContext creates a new GameContext with structure and service.
func NewGameContext(structure *GameStructure, service GameStructureServiceInterface) *GameContext {
	return &GameContext{
		Structure: structure,
		service:   service,
	}
}

// === DELEGATION METHODS ===
// These methods delegate to the service, making templates cleaner

// GetAllLocationIDs returns all location IDs recursively for this context's structure.
func (gc *GameContext) GetAllLocationIDs() []string {
	return gc.service.GetAllLocationIDs(gc.Structure)
}

// GetNextLocation returns the next location for this context's structure.
func (gc *GameContext) GetNextLocation(completedLocationIDs map[string]bool, teamID string) string {
	return gc.service.GetNextLocation(gc.Structure, completedLocationIDs, teamID)
}

// GetNextItemType returns what type of item should be next.
func (gc *GameContext) GetNextItemType(
	completedLocationIDs map[string]bool,
	completedGroupIDs map[string]bool,
) interface{} {
	return gc.service.GetNextItemType(gc.Structure, completedLocationIDs, completedGroupIDs)
}

// GetNextGroup returns the next group for this context's structure.
func (gc *GameContext) GetNextGroup(completedGroups map[string]bool) *GameContext {
	nextGroup := gc.service.GetNextGroup(gc.Structure, completedGroups)
	if nextGroup == nil {
		return nil
	}
	return NewGameContext(nextGroup, gc.service)
}

// IsCompleted checks if this context's structure is completed.
func (gc *GameContext) IsCompleted(completedCount int) bool {
	return gc.service.IsCompleted(gc.Structure, completedCount)
}

// === NAVIGATION METHODS ===

// GetSubGroupContext returns a GameContext for a specific subgroup by ID.
func (gc *GameContext) GetSubGroupContext(groupID string) *GameContext {
	subGroup := gc.service.FindGroupByID(gc.Structure, groupID)
	if subGroup == nil {
		return nil
	}
	return NewGameContext(subGroup, gc.service)
}

// GetSubGroupContexts returns GameContexts for all direct subgroups.
func (gc *GameContext) GetSubGroupContexts() []*GameContext {
	var contexts []*GameContext
	for i := range gc.Structure.SubGroups {
		contexts = append(contexts, NewGameContext(&gc.Structure.SubGroups[i], gc.service))
	}
	return contexts
}

// === CONVENIENCE ACCESSORS ===

// ID returns the structure's ID.
func (gc *GameContext) ID() string {
	return gc.Structure.ID
}

// Name returns the structure's name.
func (gc *GameContext) Name() string {
	return gc.Structure.Name
}

// Color returns the structure's color.
func (gc *GameContext) Color() string {
	return gc.Structure.Color
}

// Routing returns the structure's routing mode.
func (gc *GameContext) Routing() RouteStrategy {
	return gc.Structure.Routing
}

// Navigation returns the structure's navigation method.
func (gc *GameContext) Navigation() NavigationDisplayMode {
	return gc.Structure.Navigation
}

// CompletionType returns the structure's completion type.
func (gc *GameContext) CompletionType() CompletionType {
	return gc.Structure.CompletionType
}

// MinimumRequired returns the minimum required count.
func (gc *GameContext) MinimumRequired() int {
	return gc.Structure.MinimumRequired
}

// IsRoot returns whether this is the root group.
func (gc *GameContext) IsRoot() bool {
	return gc.Structure.IsRoot
}

// Locations returns the loaded locations.
func (gc *GameContext) Locations() []*Location {
	return gc.Structure.Locations
}

// SubGroups returns the direct subgroups (use GetSubGroupContexts for contexts).
func (gc *GameContext) SubGroups() []GameStructure {
	return gc.Structure.SubGroups
}
