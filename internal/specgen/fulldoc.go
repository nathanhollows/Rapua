package specgen

import "github.com/nathanhollows/Rapua/v7/game"

// FullSpec is the complete machine-readable specification for the v7 game format.
type FullSpec struct {
	Version  string           `json:"version"`
	Document ObjectSpec       `json:"document"`
	Enums    EnumDefs         `json:"enums"`
	Contexts []ContextDef     `json:"contexts"`
	Blocks   []game.BlockSpec `json:"blocks"`
}

// ObjectSpec describes a named object with a set of fields.
type ObjectSpec struct {
	Description string           `json:"description"`
	Fields      []game.FieldSpec `json:"fields"`
}

// EnumDefs holds all enum definitions used in the document format.
type EnumDefs struct {
	Routing    []EnumValue `json:"routing"`
	Navigation []EnumValue `json:"navigation"`
	Completion []EnumValue `json:"completion"`
}

// EnumValue is a single allowed enum value with a human-readable label and description.
type EnumValue struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// ContextDef describes a block context.
type ContextDef struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// GenerateFullSpec assembles the complete v7 spec.
func GenerateFullSpec() FullSpec {
	return FullSpec{
		Version:  "v7",
		Document: documentSpec(),
		Enums:    enumDefs(),
		Contexts: contextDefs(),
		Blocks:   GenerateBlockSpecs(),
	}
}

func documentSpec() ObjectSpec { //nolint:funlen
	structureFields := []game.FieldSpec{
		{
			Name:        "color",
			Type:        "string",
			Description: "Display colour for this group (e.g. \"primary\", \"secondary\"). Omit or empty for the root group.",
		},
		{
			Name:        "routing",
			Type:        "enum",
			Required:    true,
			Description: "How players are routed through locations. See enums.routing.",
		},
		{
			Name:        "navigation",
			Type:        "enum",
			Required:    true,
			Description: "How the navigation UI is presented. See enums.navigation.",
		},
		{
			Name:        "completion",
			Type:        "enum",
			Required:    true,
			Description: "When the group is considered complete. See enums.completion.",
		},
		{
			Name:        "minimum_required",
			Type:        "int",
			Description: "Number of locations required when completion is \"minimum\".",
		},
		{Name: "children", Type: "list", Required: true, Description: "Ordered list of location or group children.",
			Items: &game.FieldSpec{
				Type:        "object",
				Description: "Tagged union: set exactly one of \"location\" or \"group\".",
				Fields: []game.FieldSpec{
					{Name: "location", Type: "object", Description: "A single game location. See location schema."},
					{
						Name:        "group",
						Type:        "object",
						Description: "A named sub-group with its own routing/navigation/completion settings.",
					},
				},
			}},
	}

	locationFields := []game.FieldSpec{
		{
			Name:        "id",
			Type:        "string",
			Description: "Location UUID. Present on export; omit on create-import to generate a new UUID.",
		},
		{
			Name:        "slug",
			Type:        "string",
			Required:    true,
			Description: "Short alphanumeric code used in QR/URLs. Must be unique within the game.",
		},
		{Name: "name", Type: "string", Required: true, Description: "Display name shown to players."},
		{Name: "points", Type: "int", Description: "Points awarded on check-in (requires enable_points in settings)."},
		{Name: "marker", Type: "object", Description: "Geographic map pin. Omit if the location has no map position.",
			Fields: []game.FieldSpec{
				{Name: "lat", Type: "float", Required: true, Description: "Latitude in decimal degrees."},
				{Name: "lng", Type: "float", Required: true, Description: "Longitude in decimal degrees."},
			}},
		{
			Name:        "content",
			Type:        "list",
			Required:    true,
			Description: "Blocks shown to players after check-in. Always present, even if empty.",
		},
		{
			Name:        "navigation",
			Type:        "list",
			Description: "Blocks shown on the /next page to help players find this location.",
		},
	}

	blockField := game.FieldSpec{
		Type:        "object",
		Description: "Flat map with a \"type\" discriminator. See blocks for per-type fields.",
		Fields: []game.FieldSpec{
			{Name: "type", Type: "string", Required: true, Description: "Block type identifier. See blocks[].type."},
			{
				Name:        "id",
				Type:        "string",
				Description: "Block UUID. Present on export; omit on create-import to generate a new UUID.",
			},
			{Name: "points", Type: "int", Description: "Points awarded when this block is completed."},
		},
	}
	_ = blockField

	return ObjectSpec{
		Description: "Top-level v7 game document.",
		Fields: []game.FieldSpec{
			{Name: "rapua", Type: "string", Required: true, Description: "Format version. Must be \"v7\"."},
			{
				Name:        "id",
				Type:        "string",
				Description: "Instance UUID. Present on export; omit on create-import to generate a new UUID.",
			},
			{Name: "name", Type: "string", Required: true, Description: "Game name."},
			{
				Name:        "settings",
				Type:        "object",
				Required:    true,
				Description: "Game-wide settings.",
				Fields: []game.FieldSpec{
					{
						Name:        "must_check_out",
						Type:        "bool",
						Description: "Players must explicitly check out of each location before moving on.",
					},
					{Name: "show_team_count", Type: "bool", Description: "Show how many teams are at each location."},
					{Name: "enable_points", Type: "bool", Description: "Enable the points system."},
					{Name: "enable_bonus_points", Type: "bool", Description: "Enable bonus points on blocks."},
					{Name: "show_leaderboard", Type: "bool", Description: "Show the leaderboard to players."},
				},
			},
			{
				Name:        "start",
				Type:        "list",
				Required:    true,
				Description: "Blocks shown on the start page. Always present, even if empty.",
			},
			{
				Name:        "finish",
				Type:        "list",
				Required:    true,
				Description: "Blocks shown on the finish page. Always present, even if empty.",
			},
			{
				Name:        "structure",
				Type:        "object",
				Required:    true,
				Description: "Root group defining routing, navigation, and the location tree.",
				Fields:      structureFields,
			},
			{
				Name:        "location",
				Type:        "object",
				Description: "Schema for location objects within structure.children.",
				Fields:      locationFields,
			},
		},
	}
}

func enumDefs() EnumDefs {
	return EnumDefs{
		Routing: []EnumValue{
			{
				Value:       "randomised",
				Label:       "Randomised Route",
				Description: "The game randomly selects locations for each player/team. Good for large groups.",
			},
			{
				Value:       "free_roam",
				Label:       "Open Exploration",
				Description: "Players visit locations in any order. All locations are visible.",
			},
			{Value: "ordered", Label: "Guided Path", Description: "Players must visit locations in a fixed order."},
			{
				Value:       "secret",
				Label:       "Secret",
				Description: "Locations never explicitly shown; players access them out of sequence.",
			},
		},
		Navigation: []EnumValue{
			{Value: "map", Label: "Map Only", Description: "Players are shown a map with unlabelled pins."},
			{Value: "labelled_map", Label: "Labelled Map", Description: "Players are shown a map with location names."},
			{
				Value:       "location_list",
				Label:       "Location List",
				Description: "Players are shown a list of location names.",
			},
			{
				Value:       "custom",
				Label:       "Custom Clues",
				Description: "Players see custom content (e.g. randomised clues) built with the block builder.",
			},
			{
				Value:       "tasks",
				Label:       "Tasks",
				Description: "Players see a scavenger-hunt-style checklist with completion tracking.",
			},
		},
		Completion: []EnumValue{
			{Value: "all", Label: "All", Description: "All locations/groups in this group must be completed."},
			{
				Value:       "minimum",
				Label:       "Minimum",
				Description: "At least minimum_required locations/groups must be completed.",
			},
		},
	}
}

func contextDefs() []ContextDef {
	return []ContextDef{
		{Value: "location_content", Description: "Blocks shown to players after checking in to a location."},
		{
			Value:       "navigation",
			Description: "Blocks shown on the /next page to help players find a location.",
		},
		{
			Value:       "start",
			Description: "Blocks shown on the game start page (introductions, rules, team name, start button).",
		},
		{Value: "finish", Description: "Blocks shown on the game finish/end page."},
	}
}
