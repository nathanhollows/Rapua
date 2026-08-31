package specgen

import "github.com/nathanhollows/Rapua/v8/game"

// FullSpec is the complete machine-readable specification for the v8 game format.
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
	Routing    []EnumValue `json:"routing"`
	Completion []EnumValue `json:"completion"`
}

// EnumValue is a single allowed enum value with a human-readable label and description.
type EnumValue struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// BuiltInVarSpec documents a single built-in variable available in depends lists.
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

// GenerateFullSpec assembles the complete v8 spec.
func GenerateFullSpec() FullSpec {
	return FullSpec{
		Version:           "v8",
		Document:          documentSpec(),
		Enums:             enumDefs(),
		BuiltInVars:       builtInVarSpecs(),
		Contexts:          contextDefs(),
		BlockSharedFields: []game.FieldSpec{setsFieldSpec(), pointsFieldSpec()},
		Blocks:            GenerateBlockSpecs(),
	}
}

// dependsFieldSpec returns the `depends` field spec used on objectives.
func dependsFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "depends",
		Type: "list",
		Description: "Variable names gating this objective's reachability, implicitly ANDed. " +
			"Each name is a truthy check with no comparison operators; prefix a name with " +
			"\"not \" to negate it. A name is either objective.<slug> or a variable written " +
			"by a block or context \"sets\". Absent or empty means always reachable.",
		Items: &game.FieldSpec{Type: "string"},
	}
}

// setsFieldSpec returns the `sets` field spec used on interactive blocks.
func setsFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "sets",
		Type: "list",
		Description: setsSharedDescription +
			"Only valid on interactive blocks: linter emits SETS_ON_CONTENT_BLOCK warning otherwise. " +
			setsReservedDescription,
		Items: &game.FieldSpec{Type: "string"},
	}
}

// contextSetsFieldSpec returns the `sets` field spec used on an objective's
// proof and reveal contexts. A context's sets fire when every block in it
// completes, which is not the same event as any one block completing, and a
// content-only context fires them with no interactive block present at all.
// The interactive-block restriction therefore does not apply here.
func contextSetsFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "sets",
		Type: "list",
		Description: setsSharedDescription +
			"Fires once, the moment every block in this context is complete; a context with " +
			"no blocks fires immediately. " +
			setsReservedDescription,
		Items: &game.FieldSpec{Type: "string"},
	}
}

const (
	setsSharedDescription = "Variable names written on completion, as a list of names. " +
		"Sets are presence-only: each name is stored with the value \"true\". " +
		"Any other shape emits SETS_NOT_LIST. "
	setsReservedDescription = "Writing to the reserved \"objective.*\" namespace emits " +
		"SETS_RESERVED_NAMESPACE: that prefix is owned by the runtime and set automatically " +
		"when objectives complete."
)

// pointsFieldSpec returns the `points` field spec used on interactive blocks
// (RequiresValidation() true) whose own Points field is actually honoured
// (SupportsPoints() true). Content blocks (markdown, alert, divider, etc.)
// never complete, so their inherited Points field is structurally present but
// functionally inert; Broker completes but always resets Points to 0, using
// per-tier costs instead. specgen.GenerateBlockSpecs omits "points" from
// SharedFields for both reasons.
func pointsFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "points",
		Type: "int",
		Description: "Points awarded to the player when this block completes. Negative for a block framed " +
			"as a cost rather than a reward (e.g. paying points to reveal a clue). Ignored unless " +
			"settings.enable_points is true. An objective's total point value is the sum of its blocks' " +
			"points; it is not a field set on the objective itself.",
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
			Description: "How players are routed through objectives. See enums.routing.",
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
			Description: "Number of objectives required when completion is \"minimum\".",
		},
		{Name: "children", Type: "list", Required: true, Description: "Ordered list of objective or group children.",
			Items: &game.FieldSpec{
				Type:        "object",
				Description: "Tagged union: set exactly one of \"objective\" or \"group\".",
				Fields: []game.FieldSpec{
					{Name: "objective", Type: "object", Description: "A single game objective. See objective schema."},
					{
						Name:        "group",
						Type:        "object",
						Description: "A named sub-group with its own routing and completion settings.",
					},
				},
			}},
	}

	objectiveContextFields := []game.FieldSpec{
		{
			Name:        "blocks",
			Type:        "list",
			Description: "Blocks shown to players while this context is active.",
		},
		contextSetsFieldSpec(),
	}

	objectiveFields := []game.FieldSpec{
		{
			Name:        "id",
			Type:        "string",
			Description: "Objective UUID. Present on export; omit on create-import to generate a new UUID.",
		},
		{
			Name:        "slug",
			Type:        "string",
			Required:    true,
			Description: "Short alphanumeric code referenced by objective.<slug> in depends lists. Must be unique within the game.",
		},
		{Name: "title", Type: "string", Required: true, Description: "Display title shown to players."},
		dependsFieldSpec(),
		{
			Name:     "proof",
			Type:     "object",
			Required: true,
			Description: "Blocks and sets shown/fired while the objective is unproven. A non-empty proof " +
				"must contain at least one interactive block, or it gates nothing.",
			Fields: objectiveContextFields,
		},
		{
			Name:        "reveal",
			Type:        "object",
			Required:    true,
			Description: "Blocks and sets shown/fired once proof completes.",
			Fields:      objectiveContextFields,
		},
	}

	return ObjectSpec{
		Description: "Top-level v8 game document.",
		Fields: []game.FieldSpec{
			{Name: "rapua", Type: "string", Required: true, Description: "Format version. Must be \"v8\"."},
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
					{Name: "show_team_count", Type: "bool", Description: "Show how many teams are at each objective."},
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
				Description: "Root group defining routing and the objective tree.",
				Fields:      structureFields,
			},
			{
				Name: "objective",
				Type: "object",
				Description: "Schema for objective objects within structure.children. Has no points " +
					"field of its own; its total point value is the sum of its blocks' points.",
				Fields: objectiveFields,
			},
		},
	}
}

func enumDefs() EnumDefs {
	// Label/Description come from RouteStrategy/CompletionType's own String()/
	// Description() methods (game/enums.go), not a second hand-maintained copy
	// of the same text that could drift out of sync with it.
	routing := []game.RouteStrategy{
		game.RouteStrategyRandomised, game.RouteStrategyFreeRoam, game.RouteStrategyOrdered, game.RouteStrategySecret,
	}
	routingValues := make([]EnumValue, len(routing))
	for i, r := range routing {
		routingValues[i] = EnumValue{Value: string(r), Label: r.String(), Description: r.Description()}
	}

	completion := []game.CompletionType{game.CompletionAll, game.CompletionMinimum}
	completionValues := make([]EnumValue, len(completion))
	for i, c := range completion {
		completionValues[i] = EnumValue{Value: string(c), Label: c.String(), Description: c.Description()}
	}

	return EnumDefs{
		Routing:    routingValues,
		Completion: completionValues,
	}
}

// BuiltInVars returns the list of built-in variables available in depends lists.
func BuiltInVars() []BuiltInVarSpec {
	return builtInVarSpecs()
}

func builtInVarSpecs() []BuiltInVarSpec {
	return []BuiltInVarSpec{
		{
			Var:         "objective.<slug>",
			Type:        "string",
			Description: "Resolves to \"done\" when the objective with the given slug is completed, empty string otherwise.",
		},
	}
}

func contextDefs() []ContextDef {
	return []ContextDef{
		{
			Value:       "start",
			Description: "Blocks shown on the game start page (introductions, rules, team name, start button).",
		},
		{Value: "finish", Description: "Blocks shown on the game finish/end page."},
		{
			Value:       "objective_proof",
			Description: "Blocks a player must complete to prove an objective. Once every block here completes, the reveal context is shown.",
		},
		{
			Value:       "objective_reveal",
			Description: "Blocks shown once an objective's proof context is complete.",
		},
	}
}
