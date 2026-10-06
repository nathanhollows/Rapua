package specgen

import "github.com/nathanhollows/Rapua/v8/game"

// FullSpec is the complete machine-readable specification for the v8 game format.
type FullSpec struct {
	Version           string           `json:"version"`
	Document          ObjectSpec       `json:"document"`
	Enums             EnumDefs         `json:"enums"`
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
	Routing []EnumValue `json:"routing"`
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

// GenerateFullSpec assembles the complete v8 spec.
func GenerateFullSpec() FullSpec {
	return FullSpec{
		Version:           "v8",
		Document:          documentSpec(),
		Enums:             enumDefs(),
		Contexts:          contextDefs(),
		BlockSharedFields: []game.FieldSpec{pointsFieldSpec()},
		Blocks:            GenerateBlockSpecs(),
	}
}

// pointsFieldSpec returns the `points` field spec used on interactive blocks
// (RequiresValidation() true). Content blocks (markdown, alert, divider, etc.)
// never complete, so their inherited Points field is structurally present but
// functionally inert, and specgen.GenerateBlockSpecs omits "points" from their
// SharedFields.
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
	objectiveContextFields := []game.FieldSpec{
		{
			Name:        "blocks",
			Type:        "list",
			Description: "Blocks shown to players while this context is active.",
		},
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
			Description: "Short alphanumeric code identifying this objective. Must be unique within the game.",
		},
		{Name: "title", Type: "string", Required: true, Description: "Display title shown to players."},
		{
			Name: "draft",
			Type: "bool",
			Description: "Holds this objective out of play along with everything beneath it, without " +
				"moving any of them: it keeps its place and its children, so publishing restores exactly " +
				"what was there. The flag is never written downward, so a child of a draft carries none " +
				"of its own. Omitting the key leaves an existing objective's state alone; only an " +
				"explicit false publishes one. The root may not be a draft (ROOT_DRAFT), and a section " +
				"whose children are all drafts behaves as a leaf (ALL_CHILDREN_DRAFT).",
		},
		{
			Name:     "proof",
			Type:     "object",
			Required: true,
			Description: "Blocks shown while the objective is unproven. A non-empty proof " +
				"must contain at least one interactive block, or it gates nothing. Proof gates children " +
				"too: nothing below this objective is reachable until its proof clears. Clearing the " +
				"proof is what completes an objective, which is what the frontier, the journal and the " +
				"leaderboard all count. The root carries no proof or reveal content of its own " +
				"(ROOT_HAS_CONTENT).",
			Fields: objectiveContextFields,
		},
		{
			Name:     "reveal",
			Type:     "object",
			Required: true,
			Description: "Blocks shown once proof completes. On a section this is its own " +
				"content, and the section is offered to a player until they have seen it, then steps " +
				"back and its children are offered in its place.",
			Fields: objectiveContextFields,
		},
		{
			Name: "routing",
			Type: "enum",
			Description: "How players are routed through this objective's children. Required when there " +
				"are children, meaningless without them. See enums.routing.",
		},
		childrenMinFieldSpec(),
		childrenMaxFieldSpec(),
		{
			Name:        "max_next",
			Type:        "int",
			Description: "How many children a randomised objective offers at once. 0 means all of them.",
		},
		{
			Name: "finish_label",
			Type: "string",
			Description: "Label for the finish button. Only an objective in a range ever shows one, so " +
				"setting this where children_min equals children_max warns (FINISH_LABEL_UNREACHABLE).",
		},
		{
			Name: "children",
			Type: "list",
			Description: "Ordered list of child objectives, each with this same schema. An objective with " +
				"children is a section; one without is a leaf. Nothing else distinguishes them.",
		},
	}

	return ObjectSpec{
		Description: "Top-level v8 game document.",
		Fields: []game.FieldSpec{
			{Name: "rapua", Type: "string", Required: true, Description: "Format version. Must be \"v8\"."},
			{
				Name:        "id",
				Type:        "string",
				Description: "Quest UUID. Present on export; omit on create-import to generate a new UUID.",
			},
			{Name: "name", Type: "string", Required: true, Description: "Game name."},
			{
				Name:        "settings",
				Type:        "object",
				Required:    true,
				Description: "Game-wide settings.",
				Fields: []game.FieldSpec{
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
				Name:     "structure",
				Type:     "object",
				Required: true,
				Description: "The root objective. An ordinary objective with no parent, using the schema " +
					"below: the tree is one recursive type, not a container wrapping a different one.",
				Fields: objectiveFields,
			},
			{
				Name: "objective",
				Type: "object",
				Description: "Schema for every objective, root and children alike. Has no points field of " +
					"its own; its total point value is the sum of its blocks' points.",
				Fields: objectiveFields,
			},
		},
	}
}

// childrenMinFieldSpec and childrenMaxFieldSpec document the completion band as
// the single rule it is, rather than as two independent integers: the
// relationship between the two bounds is what decides the node's behaviour, and
// it is the thing authors get wrong.
func childrenMinFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "children_min",
		Type: "int",
		Description: "How many children must complete before the player may finish this objective. " +
			bandRuleDescription,
	}
}

func childrenMaxFieldSpec() game.FieldSpec {
	return game.FieldSpec{
		Name: "children_max",
		Type: "int",
		Description: "How many completed children finish this objective on their own. " +
			bandRuleDescription,
	}
}

const bandRuleDescription = "The pair forms a completion band. When min equals max the objective " +
	"auto-completes at that count with no player action. When min is lower, reaching min only reveals " +
	"a finish button and the player's press is what completes the objective, which also auto-completes " +
	"at max. Omitting both means every child is required ([n, n]). Naming either bound widens the other " +
	"to its extreme: an omitted min is 0, an omitted max is the child count. So an explicit " +
	"children_min of 0 is not the same as omitting it. min greater than max is an error " +
	"(BAND_MIN_EXCEEDS_MAX), as is either bound outside 0..child count (BAND_OUT_OF_RANGE). Both are " +
	"meaningless on an objective with no children (BAND_ON_LEAF)."

func enumDefs() EnumDefs {
	// Label/Description come from RouteStrategy's own String()/Description()
	// methods (game/enums.go), not a second hand-maintained copy of the same
	// text that could drift out of sync with it.
	routing := []game.RouteStrategy{
		game.RouteStrategyRandomised, game.RouteStrategyFreeRoam, game.RouteStrategyOrdered,
	}
	routingValues := make([]EnumValue, len(routing))
	for i, r := range routing {
		routingValues[i] = EnumValue{Value: string(r), Label: r.String(), Description: r.Description()}
	}

	return EnumDefs{Routing: routingValues}
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
