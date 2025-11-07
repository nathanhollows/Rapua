package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Instance represents an entire game state.
type m20251030094613_instance struct {
	bun.BaseModel `bun:"table:instances"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID                    string                     `bun:"id,pk,type:varchar(36)"`
	Name                  string                     `bun:"name,type:varchar(255)"`
	UserID                string                     `bun:"user_id,type:varchar(36)"`
	IsTemplate            bool                       `bun:"is_template,type:bool"`
	TemplateID            string                     `bun:"template_id,type:varchar(36),nullzero"`
	StartTime             bun.NullTime               `bun:"start_time,nullzero"`
	EndTime               bun.NullTime               `bun:"end_time,nullzero"`
	Status                m20241209083639_GameStatus `bun:"-"`
	IsQuickStartDismissed bool                       `bun:"is_quick_start_dismissed,type:bool"`
	GameStructure         string                     `bun:"game_structure,type:text"` // JSON stored as text

	Teams      []m20241209090041_Team            `bun:"rel:has-many,join:id=instance_id"`
	Locations  []m20251030094613_location        `bun:"rel:has-many,join:id=instance_id"`
	Settings   *m20251030094613_instanceSettings `bun:"rel:has-one,join:id=instance_id"`
	ShareLinks []m20250224085041_ShareLink       `bun:"rel:has-many,join:id=template_id"`
}

type m20251030094613_location struct {
	bun.BaseModel `bun:"table:locations"`

	ID         string `bun:"id,pk,notnull"`
	InstanceID string `bun:"instance_id,notnull"`
	Name       string `bun:"name,type:varchar(255)"`
}

type m20251030094613_Team struct {
	bun.BaseModel `bun:"table:teams"`

	ID              string   `bun:"id,pk"`
	Code            string   `bun:"code,unique"`
	Name            string   `bun:"name,"`
	InstanceID      string   `bun:"instance_id,notnull"`
	HasStarted      bool     `bun:"has_started,default:false"`
	MustCheckOut    string   `bun:"must_scan_out"`
	Points          int      `bun:"points,"`
	SkippedGroupIDs []string `bun:"skipped_group_ids,type:text[],array"`
}

type m20251030094613_instanceSettings struct {
	bun.BaseModel `bun:"table:instance_settings"`

	InstanceID            string                                `bun:"instance_id,pk,type:varchar(36)"`
	RouteStrategy         m20250927085100_RouteStrategy         `bun:"navigation_mode,type:int"`
	NavigationDisplayMode m20250927085100_NavigationDisplayMode `bun:"navigation_method,type:int"`
}

// Timestamped copy of enum types for this migration.
type m20251030094613_RouteStrategy int
type m20251030094613_NavigationDisplayMode int

const (
	m20251030094613_RouteStrategyRandom m20251030094613_RouteStrategy = iota
	m20251030094613_RouteStrategyFreeRoam
	m20251030094613_RouteStrategyOrdered
)

const (
	m20251030094613_NavigationDisplayMap m20251030094613_NavigationDisplayMode = iota
	m20251030094613_NavigationDisplayMapAndNames
	m20251030094613_NavigationDisplayNames
	m20251030094613_NavigationDisplayClues
	m20251030094613_NavigationDisplayCustom
)

// Timestamped copy of CompletionType for this migration.
type m20251030094613_CompletionType string

const (
	m20251030094613_CompletionAll     m20251030094613_CompletionType = "all"
	m20251030094613_CompletionMinimum m20251030094613_CompletionType = "minimum"
)

// Timestamped copy of GameStructure for this migration
// This ensures the migration always uses the structure as it was at the time of creation,
// regardless of future changes to the live models.GameStructure.
type m20251030094613_GameStructure struct {
	ID              string                                `json:"id"`
	Name            string                                `json:"name"`
	Color           string                                `json:"color"`
	Routing         m20251030094613_RouteStrategy         `json:"routing"`
	Navigation      m20251030094613_NavigationDisplayMode `json:"navigation"`
	CompletionType  m20251030094613_CompletionType        `json:"completion_type"`
	MinimumRequired int                                   `json:"minimum_required,omitempty"`
	AutoAdvance     bool                                  `json:"auto_advance"` // NEW: Auto-advance when completion criteria met
	IsRoot          bool                                  `json:"is_root"`
	LocationIDs     []string                              `json:"location_ids"`
	SubGroups       []m20251030094613_GameStructure       `json:"sub_groups"`
}

func init() {
	// Adds the GameStructure field, migrates existing location data, and creates team_progress table
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Add the game_structure column
		_, err := db.NewAddColumn().
			Model((*m20251030094613_instance)(nil)).
			ColumnExpr("game_structure text").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("20251030094613_location_groups.go: add column game_structure: %w", err)
		}

		// Get all instances with their locations and settings
		var instances []m20251030094613_instance
		err = db.NewSelect().
			Model(&instances).
			Relation("Locations").
			Relation("Settings").
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("20251030094613_location_groups.go: fetch instances: %w", err)
		}

		// For each instance, create a root GameStructure
		for _, instance := range instances {
			// Create location ID array from locations (preserves order)
			locationIDs := make([]string, len(instance.Locations))
			for i, location := range instance.Locations {
				locationIDs[i] = location.ID
			}

			// Default navigation settings if settings don't exist
			routing := m20251030094613_RouteStrategyFreeRoam
			navigation := m20251030094613_NavigationDisplayCustom
			if instance.Settings != nil {
				routing = m20251030094613_RouteStrategy(instance.Settings.RouteStrategy)
				navigation = m20251030094613_NavigationDisplayMode(instance.Settings.NavigationDisplayMode)
			}

			// Create a default visible group containing all locations
			defaultGroup := m20251030094613_GameStructure{
				ID:             uuid.New().String(),
				Name:           "Locations",
				Color:          "primary",
				Routing:        routing,
				Navigation:     navigation,
				CompletionType: m20251030094613_CompletionAll,
				IsRoot:         false,
				LocationIDs:    locationIDs,
				SubGroups:      []m20251030094613_GameStructure{},
			}

			// Create the root GameStructure (unnamed container, not rendered)
			// The root group contains all visible groups and ungrouped locations
			rootStructure := m20251030094613_GameStructure{
				ID:             uuid.New().String(),
				Name:           "", // Root is always unnamed
				Color:          "",
				Routing:        routing,
				Navigation:     navigation,
				CompletionType: m20251030094613_CompletionAll,
				IsRoot:         true,
				LocationIDs:    []string{}, // Root can have ungrouped locations
				SubGroups:      []m20251030094613_GameStructure{defaultGroup},
			}

			// Marshal to JSON
			jsonData, err := json.Marshal(rootStructure)
			if err != nil {
				return fmt.Errorf(
					"20251030094613_location_groups.go: marshal game_structure for instance %s: %w",
					instance.ID,
					err,
				)
			}

			// Update the instance with the new game_structure
			_, err = db.NewUpdate().
				Model((*m20251030094613_instance)(nil)).
				Set("game_structure = ?", string(jsonData)).
				Where("id = ?", instance.ID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf(
					"20251030094613_location_groups.go: update game_structure for instance %s: %w",
					instance.ID,
					err,
				)
			}
		}

		// Add skipped_group_ids column to teams table
		_, err = db.NewAddColumn().
			Model((*m20251030094613_Team)(nil)).
			ColumnExpr("skipped_group_ids text default '[]'").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("20251030094613_location_groups.go: add skipped_group_ids column: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration - drop the game_structure column and skipped_group_ids
		// Drop skipped_group_ids column from teams table
		_, err := db.NewDropColumn().
			Model((*m20251030094613_Team)(nil)).
			Column("skipped_group_ids").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("20251030094613_location_groups.go: drop skipped_group_ids column: %w", err)
		}

		// Drop game_structure column from instances table
		_, err = db.NewDropColumn().
			Model((*m20251030094613_instance)(nil)).
			Column("game_structure").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("20251030094613_location_groups.go: drop column game_structure: %w", err)
		}

		return nil
	})
}
