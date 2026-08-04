package specgen

import "github.com/nathanhollows/Rapua/v7/game"

// FullSpec is the complete machine-readable specification for the v7 game format.
type FullSpec struct {
	Version           string           `json:"version"`
	Document          ObjectSpec       `json:"document"`
	Enums             EnumDefs         `json:"enums"`
	BuiltInVars       []BuiltInVarSpec `json:"built_in_vars"`
	Contexts          []ContextDef     `json:"contexts"`
	BlockSharedFields []game.FieldSpec `json:"block_shared_fields"` // fields shared by all/some blocks; referenced by name in BlockSpec.shared_fields
	Blocks            []game.BlockSpec `json:"blocks"`
}

// ObjectSpec describes a named object with a set of fields.
type ObjectSpec struct {
	Description string           `json:"description"`
	Fields      []game.FieldSpec `json:"fields"`
}

// EnumDefs holds all enum definitions used in the document format.
type EnumDefs struct {
	Routing      []EnumValue `json:"routing"`
	Completion   []EnumValue `json:"completion"`
	ConditionOps []EnumValue `json:"condition_ops"`
}

// EnumValue is a single allowed enum value with a human-readable label and description.
type EnumValue struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// BuiltInVarSpec documents a single built-in variable available in when clauses.
type BuiltInVarSpec struct {
	Var         string `json:"var"`
	Type        string `json:"type"`
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
		Version:           "v7",
		Document:          documentSpec(),
		Enums:             enumDefs(),
		BuiltInVars:       builtInVarSpecs(),
		Contexts:          contextDefs(),
		BlockSharedFields: []game.FieldSpec{whenFieldSpec(), setsFieldSpec()},
		Blocks:            GenerateBlockSpecs(),
	}
}

// whenFieldSpec returns the shared `when` field spec used on blocks, locations, and groups.
func whenFieldSpec() game.FieldSpec {
	condItem := &game.FieldSpec{
		Type:        "object",
		Description: "A single condition. var is required; op+value are optional comparisons; not negates the result.",
		Fields: []game.FieldSpec{
			{
				Name:        "var",
				Type:        "string",
				Required:    true,
				Description: "Variable to check. Built-in: player.points, run.started_at, location.<slug>.visited, location.<slug>.checked_in, group.<name>.completed, game.team_count. Creator-defined via block sets.",
			},
			{
				Name:        "op",
				Type:        "enum",
				Description: "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
				Enum:        []string{"eq", "neq", "gt", "lt", "gte", "lte", "in", "not_in"},
			},
			{
				Name:        "value",
				Type:        "any",
				Description: "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present.",
			},
			{
				Name:        "not",
				Type:        "bool",
				Description: "Negate the result of this condition.",
			},
		},
	}
	return game.FieldSpec{
		Name:        "when",
		Type:        "object",
		Description: "Visibility conditions. Element is hidden when conditions are not met. Absent means always visible.",
		Fields: []game.FieldSpec{
			{
				Name:        "all_of",
				Type:        "list",
				Description: "ALL conditions must be true (AND). Each item is a condition object.",
				Items:       condItem,
			},
			{
				Name:        "any_of",
				Type:        "list",
				Description: "At least one condition must be true (OR). Each item is a condition object.",
				Items:       condItem,
			},
		},
	}
}

// setsFieldSpec returns the `sets` field spec used on interactive blocks.
func setsFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "sets",
		Type: "object",
		Description: "Variables written when this block completes, as an object of {name: value}. " +
			"Values may be strings, numbers, or booleans; all are stored as strings. " +
			"Any other shape emits SETS_NOT_OBJECT. " +
			"Only valid on interactive blocks — linter emits SETS_ON_CONTENT_BLOCK warning otherwise.",
		Items: &game.FieldSpec{Type: "string"},
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
		whenFieldSpec(),
		{Name: "children", Type: "list", Required: true, Description: "Ordered list of location or group children.",
			Items: &game.FieldSpec{
				Type:        "object",
				Description: "Tagged union: set exactly one of \"location\" or \"group\".",
				Fields: []game.FieldSpec{
					{Name: "location", Type: "object", Description: "A single game location. See location schema."},
					{
						Name:        "group",
						Type:        "object",
						Description: "A named sub-group with its own routing and completion settings.",
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
		whenFieldSpec(),
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
				Description: "Root group defining routing and the location tree.",
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
		Completion: []EnumValue{
			{Value: "all", Label: "All", Description: "All locations/groups in this group must be completed."},
			{
				Value:       "minimum",
				Label:       "Minimum",
				Description: "At least minimum_required locations/groups must be completed.",
			},
		},
		ConditionOps: []EnumValue{
			{Value: "eq", Label: "Equal", Description: "var == value"},
			{Value: "neq", Label: "Not equal", Description: "var != value"},
			{Value: "gt", Label: "Greater than", Description: "var > value (numeric)"},
			{Value: "lt", Label: "Less than", Description: "var < value (numeric)"},
			{Value: "gte", Label: "Greater than or equal", Description: "var >= value (numeric)"},
			{Value: "lte", Label: "Less than or equal", Description: "var <= value (numeric)"},
			{Value: "in", Label: "In array", Description: "var is one of value (value must be an array)"},
			{Value: "not_in", Label: "Not in array", Description: "var is not in value (value must be an array)"},
		},
	}
}

// BuiltInVars returns the list of built-in variables available in when-clause conditions.
func BuiltInVars() []BuiltInVarSpec {
	return builtInVarSpecs()
}

func builtInVarSpecs() []BuiltInVarSpec {
	return []BuiltInVarSpec{
		{
			Var:         "player.points",
			Type:        "int",
			Description: "Total points earned on this run. Evaluated live from the run's points.",
		},
		{
			Var:         "points",
			Type:        "int",
			Description: "Pre-respine spelling of player.points. Still resolves; prefer player.points.",
		},
		{
			Var:         "run.started_at",
			Type:        "timestamp",
			Description: "RFC3339 timestamp of when the run began. Empty until the players start.",
		},
		{
			Var:         "location.<slug>.visited",
			Type:        "bool",
			Description: "True when the team has a CheckIn record for the location with the given slug.",
		},
		{
			Var:         "location.<slug>.checked_in",
			Type:        "bool",
			Description: "True when the team has checked in and BlocksCompleted is true for the given location slug.",
		},
		{
			Var:         "group.<name>.completed",
			Type:        "bool",
			Description: "True when all required locations in the named group have been visited and completed.",
		},
		{
			Var:         "game.team_count",
			Type:        "int",
			Description: "Number of teams with HasStarted == true in this game instance.",
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
