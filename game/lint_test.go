package game_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRegistry struct {
	validTypes  map[string]bool
	contexts    map[string][]game.BlockContext
	knownFields map[string][]string
	interactive map[string]bool
	docSetsVars map[string][]string // block type → var names returned by DocSetsVars
}

func (m *mockRegistry) IsValidType(blockType string) bool {
	return m.validTypes[blockType]
}

func (m *mockRegistry) CanUseInContext(blockType string, ctx game.BlockContext) bool {
	ctxs, ok := m.contexts[blockType]
	if !ok {
		return false
	}
	return slices.Contains(ctxs, ctx)
}

func (m *mockRegistry) KnownFields(t string) []string {
	if m.knownFields == nil {
		return nil
	}
	return m.knownFields[t]
}

func (m *mockRegistry) IsInteractive(blockType string) bool {
	return m.interactive[blockType]
}

func (m *mockRegistry) DocSetsVars(blockType string, _ game.BlockDoc) []string {
	if m.docSetsVars == nil {
		return nil
	}
	return m.docSetsVars[blockType]
}

func (m *mockRegistry) ValidateBlock(_, _ string, _ game.BlockDoc) ([]game.LintDiag, []game.LintDiag) {
	return nil, nil
}

func newTestRegistry() *mockRegistry {
	return &mockRegistry{
		validTypes: map[string]bool{
			"text":         true,
			"clue":         true,
			"quiz":         true,
			"choice":       true,
			"start_button": true,
			"game_status":  true,
			"password":     true,
		},
		contexts: map[string][]game.BlockContext{
			"text": {
				game.ContextStart, game.ContextFinish,
				game.ContextObjectiveProof, game.ContextObjectiveReveal,
			},
			"clue": {game.ContextObjectiveProof, game.ContextObjectiveReveal},
			"quiz": {
				game.ContextObjectiveProof, game.ContextObjectiveReveal,
			},
			"choice": {
				game.ContextObjectiveProof, game.ContextObjectiveReveal,
			},
			"start_button": {game.ContextStart},
			"game_status":  {game.ContextStart},
		},
		interactive: map[string]bool{
			"quiz":     true,
			"choice":   true,
			"password": true,
		},
	}
}

// validDoc returns a valid doc whose sole child is an objective, wrapped in a
// group (a bare root-level objective triggers ROOT_OBJECTIVE_HIDDEN).
func validDoc() *game.GameDoc {
	return &game.GameDoc{
		Rapua: "v8",
		Name:  "Test Game",
		Settings: game.SettingsDoc{
			EnablePoints: true,
		},
		Start: []game.BlockDoc{
			{"type": "start_button"},
		},
		Finish: []game.BlockDoc{},
		Structure: game.StructureDoc{
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Group: &game.GroupDoc{
					Name:       "Stage One",
					Color:      "primary",
					Routing:    game.RouteStrategyFreeRoam,
					Completion: game.CompletionAll,
					Children: []game.ChildDoc{
						{Objective: &game.ObjectiveDoc{
							Slug:  "lobby",
							Title: "The Lobby",
							Proof: game.ObjectiveContextDoc{
								Blocks: []game.BlockDoc{{"type": "quiz"}},
							},
						}},
					},
				}},
			},
		},
	}
}

func TestLint_ValidDoc(t *testing.T) {
	doc := validDoc()
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
}

// --- Layer 1: Schema ---

func TestLint_WrongVersion(t *testing.T) {
	doc := validDoc()
	doc.Rapua = "v6"
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "VERSION_MISMATCH", result.Errors[0].Code)
}

func TestLint_MissingName(t *testing.T) {
	doc := validDoc()
	doc.Name = ""
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_NAME", result.Errors[0].Code)
}

func TestLint_InvalidRouting(t *testing.T) {
	doc := validDoc()
	doc.Structure.Routing = "bogus"
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_ROUTING", result.Errors[0].Code)
}

func TestLint_UnknownBlockType(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "nonexistent_block"}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "UNKNOWN_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_MinimumRequiredMismatch(t *testing.T) {
	doc := validDoc()
	doc.Structure.Completion = game.CompletionAll
	doc.Structure.MinimumRequired = 2
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MINIMUM_REQUIRED_MISMATCH", result.Errors[0].Code)
}

func TestLint_MinimumRequiredMissing(t *testing.T) {
	doc := validDoc()
	doc.Structure.Completion = game.CompletionMinimum
	doc.Structure.MinimumRequired = 0
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MINIMUM_REQUIRED_MISSING", result.Errors[0].Code)
}

// --- Layer 2: Semantic ---

func TestLint_DuplicateSlugs(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Objective: &game.ObjectiveDoc{
			Slug:  "lobby", // duplicate.
			Title: "Another Lobby",
		},
	})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "SLUG_DUPLICATE", result.Errors[0].Code)
}

func TestLint_InvalidContext(t *testing.T) {
	doc := validDoc()
	// quiz can't be in start context.
	doc.Start = append(doc.Start, game.BlockDoc{"type": "quiz"})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_CONTEXT", result.Errors[0].Code)
}

// --- Layer 3: Structural warnings ---

func TestLint_EmptyGroup(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "Empty Group",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children:   []game.ChildDoc{},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "EMPTY_GROUP", result.Warnings[0].Code)
}

func TestLint_NoStartButton(t *testing.T) {
	doc := validDoc()
	doc.Start = []game.BlockDoc{{"type": "text"}} // no start_button
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "NO_START_BUTTON", result.Warnings[0].Code)
}

func TestLint_PointsDisabled(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "points": float64(10)}},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

// --- IsValid ---

func TestLintResult_IsValid(t *testing.T) {
	r := game.LintResult{}
	assert.True(t, r.IsValid())
	r.Errors = append(r.Errors, game.LintDiag{Code: "FOO"})
	assert.False(t, r.IsValid())
}

// --- Schema: finish blocks, group name, empty child ---

func TestLint_ValidDocWithFinishBlock(t *testing.T) {
	doc := validDoc()
	doc.Finish = []game.BlockDoc{{"type": "text"}}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
}

func TestLint_MissingGroupName(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children:   []game.ChildDoc{},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_GROUP_NAME", result.Errors[0].Code)
}

func TestLint_EmptyChild(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "EMPTY_CHILD", result.Errors[0].Code)
}

func TestLint_InvalidCompletion(t *testing.T) {
	doc := validDoc()
	doc.Structure.Completion = "bogus_completion"
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_COMPLETION", result.Errors[0].Code)
}

// --- Schema: block type checks ---

func TestLint_MissingBlockType(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{}}, // no "type" key.
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_InvalidBlockTypeNotString(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": 123}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_NegativeBlockPoints(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "points": float64(-5)}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_POINTS", result.Errors[0].Code)
}

func TestLint_NegativeBlockPointsJsonNumber(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "points": json.Number("-5")}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_POINTS", result.Errors[0].Code)
}

func TestLint_UnknownField(t *testing.T) {
	reg := newTestRegistry()
	reg.knownFields = map[string][]string{
		"text": {"content"},
	}
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "bogus_field": "value"}},
	}
	result := game.Lint(doc, reg)
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "UNKNOWN_FIELD", result.Warnings[0].Code)
}

func TestLint_NilRegistry(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "any_type"}},
	}
	result := game.Lint(doc, nil)
	assert.Empty(t, result.Errors)
}

// --- Semantic: group slug deduplication, block ID duplicates ---

func TestLint_DuplicateSlugInGroup(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "Group",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Objective: &game.ObjectiveDoc{
					Slug:  "lobby", // duplicate of top-level objective.
					Title: "Lobby Copy",
				}},
			},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "SLUG_DUPLICATE", result.Errors[0].Code)
}

func TestLint_DuplicateSlugInNestedGroup(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "Outer",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Group: &game.GroupDoc{
					Name:       "Inner",
					Color:      "secondary",
					Routing:    game.RouteStrategyFreeRoam,
					Completion: game.CompletionAll,
					Children: []game.ChildDoc{
						{Objective: &game.ObjectiveDoc{
							Slug:  "lobby", // duplicate of top-level objective.
							Title: "Deep Lobby",
						}},
					},
				}},
			},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "SLUG_DUPLICATE", result.Errors[0].Code)
}

func TestLint_BlockIDDuplicate(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{
			{"type": "text", "id": "block-abc"},
			{"type": "text", "id": "block-abc"},
		},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "BLOCK_ID_DUPLICATE", result.Errors[0].Code)
}

// --- Structural: nested group, group-level points disabled ---

func TestLint_NestedEmptyGroupWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "Outer Group",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Group: &game.GroupDoc{
					Name:       "Inner Empty",
					Color:      "secondary",
					Routing:    game.RouteStrategyFreeRoam,
					Completion: game.CompletionAll,
					Children:   []game.ChildDoc{},
				}},
			},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "EMPTY_GROUP", result.Warnings[0].Code)
}

func TestLint_PointsDisabledJsonNumber(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "points": json.Number("10")}},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

func TestLint_PointsDisabledInNestedGroup(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "Outer",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Group: &game.GroupDoc{
					Name:       "Inner",
					Color:      "secondary",
					Routing:    game.RouteStrategyFreeRoam,
					Completion: game.CompletionAll,
					Children: []game.ChildDoc{
						{Objective: &game.ObjectiveDoc{
							Slug:  "deep-station",
							Title: "Deep Station",
							Reveal: game.ObjectiveContextDoc{
								Blocks: []game.BlockDoc{{"type": "text", "points": float64(10)}},
							},
						}},
					},
				}},
			},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	// POINTS_DISABLED for deep-station's reveal block.
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

func TestLint_RootObjectiveHidden(t *testing.T) {
	// Objectives placed directly under structure.children (not inside a group) are never shown.
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Objective: &game.ObjectiveDoc{
			Slug:  "orphan",
			Title: "Orphan Objective",
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "ROOT_OBJECTIVE_HIDDEN", result.Warnings[0].Code)
}

func TestLint_RootHasNoGroups(t *testing.T) {
	// Structure with only root-level objectives and no groups at all: all objectives hidden.
	doc := validDoc()
	doc.Structure.Children = []game.ChildDoc{
		{Objective: &game.ObjectiveDoc{Slug: "stop-a", Title: "Stop A"}},
		{Objective: &game.ObjectiveDoc{Slug: "stop-b", Title: "Stop B"}},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	// One ROOT_OBJECTIVE_HIDDEN per bare objective.
	require.Len(t, result.Warnings, 2)
	assert.Equal(t, "ROOT_OBJECTIVE_HIDDEN", result.Warnings[0].Code)
	assert.Equal(t, "ROOT_OBJECTIVE_HIDDEN", result.Warnings[1].Code)
}

// --- When / variable resolution ---

func TestLint_UndefinedVar_ObjectiveDepends(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"ghost_var"}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

// A negated name is still a reference, so a typo behind "not " is caught too.
func TestLint_UndefinedVar_NegatedDepends(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"not ghost_var"}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

func TestLint_DependsEmptyName_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"   "}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_EMPTY_NAME"))
}

func TestLint_DependsSelfReference_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"objective.lobby"}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_CYCLE"))
}

func TestLint_DependsMutualCycle_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"objective.annex"}
	doc.Structure.Children[0].Group.Children = append(
		doc.Structure.Children[0].Group.Children,
		game.ChildDoc{Objective: &game.ObjectiveDoc{
			Slug:    "annex",
			Title:   "The Annex",
			Depends: game.DependsField{"objective.lobby"},
		}},
	)
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_CYCLE"))
}

// Negation does not break a cycle: "not other" still cannot be evaluated until
// other has been, so the chain is just as unreachable.
func TestLint_DependsNegatedCycle_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"not objective.lobby"}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_CYCLE"))
}

// A diamond is not a cycle: two objectives may both gate on a third.
func TestLint_DependsDiamond_NoError(t *testing.T) {
	doc := validDoc()
	for _, slug := range []string{"left", "right"} {
		doc.Structure.Children[0].Group.Children = append(
			doc.Structure.Children[0].Group.Children,
			game.ChildDoc{Objective: &game.ObjectiveDoc{
				Slug:    slug,
				Title:   slug,
				Depends: game.DependsField{"objective.lobby"},
			}},
		)
	}
	result := game.Lint(doc, newTestRegistry())
	assert.False(t, result.HasError("DEPENDS_CYCLE"))
}

func TestLint_DefinedVar_NoWarning(t *testing.T) {
	doc := validDoc()
	// Block sets "score", a sibling objective's depends references it: clean.
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{
			{
				"type": "quiz",
				"sets": []any{"score"},
			},
		},
	}
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"score"}
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_VAR", w.Code)
	}
}

func TestLint_UnusedVar(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": []any{"score"}}},
	}
	// "score" is set but no depends list references it.
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNUSED_VAR")
}

func TestLint_UnusedVar_UsedElsewhere_NoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": []any{"score"}}},
	}
	// A second objective's depends references "score": should suppress UNUSED_VAR.
	secondObj := &game.ObjectiveDoc{
		Slug:    "loc2",
		Title:   "Location 2",
		Depends: game.DependsField{"score"},
	}
	doc.Structure.Children[0].Group.Children = append(
		doc.Structure.Children[0].Group.Children,
		game.ChildDoc{Objective: secondObj},
	)
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNUSED_VAR", w.Code)
	}
}

// --- Non-interactive block sets tests ---

func TestLint_SetsOnNonInteractiveBlock_Warning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "sets": []any{"foo"}}},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "SETS_ON_CONTENT_BLOCK")
}

// A wrong-shaped "sets" must be caught by lint, with a path, rather than
// surfacing later as an unmarshalling error with no location.
func TestLint_SetsAsObject_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": map[string]any{"found_clue": "true"}}},
	}
	result := game.Lint(doc, newTestRegistry())

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	require.Contains(t, codes, "SETS_NOT_LIST")

	for _, e := range result.Errors {
		if e.Code == "SETS_NOT_LIST" {
			assert.Contains(t, e.Path, ".sets")
			assert.Contains(t, e.Message, "must be a list")
		}
	}
}

func TestLint_SetsAsScalar_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": "found_clue"}},
	}
	result := game.Lint(doc, newTestRegistry())

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SETS_NOT_LIST")
}

// A list holding anything but names is as unusable as an object.
func TestLint_SetsAsNonStringList_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": []any{1, 2}}},
	}
	result := game.Lint(doc, newTestRegistry())

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SETS_NOT_LIST")
}

func TestLint_SetsReservedNamespace_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": []any{"objective.find-maisie"}}},
	}
	result := game.Lint(doc, newTestRegistry())

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	require.Contains(t, codes, "SETS_RESERVED_NAMESPACE")

	for _, e := range result.Errors {
		if e.Code == "SETS_RESERVED_NAMESPACE" {
			assert.Contains(t, e.Path, ".sets")
			assert.Contains(t, e.Message, "reserved namespace")
		}
	}
}

func TestLint_SetsReservedNamespace_RegistryPath_Error(t *testing.T) {
	// Registry-sourced vars (e.g. choice block options[*].sets) must also
	// be checked for reserved namespaces.
	reg := newTestRegistry()
	reg.docSetsVars = map[string][]string{"choice": {"objective.find-maisie"}}

	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "choice", "options": []any{
			map[string]any{"label": "Yes", "sets": "objective.find-maisie"},
		}}},
	}
	result := game.Lint(doc, reg)

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	require.Contains(t, codes, "SETS_RESERVED_NAMESPACE")

	for _, e := range result.Errors {
		if e.Code == "SETS_RESERVED_NAMESPACE" {
			assert.Contains(t, e.Path, "proof.blocks[0]") // block path, not .sets.
			assert.Contains(t, e.Message, "reserved namespace")
		}
	}
}

// The shape check does not depend on the block type resolving, so a block that
// is broken in two ways reports both faults rather than only the first.
func TestLint_SetsShapeChecked_WhenTypeIsUnusable(t *testing.T) {
	for _, tt := range []struct {
		name  string
		block game.BlockDoc
		want  string
	}{
		{
			name:  "unknown type",
			block: game.BlockDoc{"type": "not_a_block", "sets": map[string]any{"found_clue": "true"}},
			want:  "UNKNOWN_BLOCK_TYPE",
		},
		{
			name:  "missing type",
			block: game.BlockDoc{"sets": map[string]any{"found_clue": "true"}},
			want:  "MISSING_BLOCK_TYPE",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDoc()
			doc.Structure.Children[0].Group.Children[0].Objective.Reveal = game.ObjectiveContextDoc{
				Blocks: []game.BlockDoc{tt.block},
			}
			result := game.Lint(doc, newTestRegistry())

			codes := make([]string, len(result.Errors))
			for i, e := range result.Errors {
				codes[i] = e.Code
			}
			assert.Contains(t, codes, tt.want)
			assert.Contains(t, codes, "SETS_NOT_LIST")
		})
	}
}

// --- LintJSON unknown field tests ---

func TestLintJSON_UnknownFieldInGameDoc(t *testing.T) {
	data := []byte(`{
		"rapua": "v8",
		"name": "Test",
		"hallucinated_field": true,
		"settings": {},
		"start": [],
		"finish": [],
		"structure": {"routing": "free_roam", "completion": "all", "children": []}
	}`)
	result := game.LintJSON(data, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNKNOWN_FIELD")
}

func TestLintJSON_UnknownFieldInObjective(t *testing.T) {
	data := []byte(`{
		"rapua": "v8",
		"name": "Test",
		"settings": {},
		"start": [],
		"finish": [],
		"structure": {
			"routing": "free_roam",
			"completion": "all",
			"children": [{
				"group": {
					"name": "Group A",
					"routing": "free_roam",
					"completion": "all",
					"children": [{
						"objective": {
							"slug": "loc-a",
							"title": "Loc A",
							"proof": {},
							"reveal": {},
							"ai_added_field": "oops"
						}
					}]
				}
			}]
		}
	}`)
	result := game.LintJSON(data, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNKNOWN_FIELD")
}

func TestLintJSON_UnknownFieldInGroup(t *testing.T) {
	data := []byte(`{
		"rapua": "v8",
		"name": "Test",
		"settings": {},
		"start": [],
		"finish": [],
		"structure": {
			"routing": "free_roam",
			"completion": "all",
			"children": [{
				"group": {
					"name": "Group A",
					"routing": "free_roam",
					"completion": "all",
					"children": [],
					"ai_added_field": "oops"
				}
			}]
		}
	}`)
	result := game.LintJSON(data, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNKNOWN_FIELD")
}

func TestLintJSON_ValidDoc_NoUnknownFieldWarnings(t *testing.T) {
	doc := validDoc()
	data, err := json.Marshal(doc)
	require.NoError(t, err)
	result := game.LintJSON(data, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNKNOWN_FIELD", w.Code,
			"unexpected UNKNOWN_FIELD warning: %s at %s", w.Message, w.Path)
	}
}

// --- SLUG_INVALID_FORMAT ---

func TestLint_SlugInvalidFormat_Uppercase(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Slug = "The-Lobby"
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_INVALID_FORMAT")
}

func TestLint_SlugInvalidFormat_LeadingHyphen(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Slug = "-lobby"
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_INVALID_FORMAT")
}

func TestLint_SlugValidFormat_NoError(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Slug = "the-lobby-2"
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "SLUG_INVALID_FORMAT", e.Code)
	}
}

// --- MINIMUM_REQUIRED_EXCEEDS_CHILDREN ---

func TestLint_MinimumRequiredExceedsChildren_Group(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Completion = game.CompletionMinimum
	doc.Structure.Children[0].Group.MinimumRequired = 5
	// Group has 1 child.
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "MINIMUM_REQUIRED_EXCEEDS_CHILDREN")
}

func TestLint_MinimumRequiredExceedsChildren_Structure(t *testing.T) {
	doc := validDoc()
	doc.Structure.Completion = game.CompletionMinimum
	doc.Structure.MinimumRequired = 99
	// Structure has 1 child.
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "MINIMUM_REQUIRED_EXCEEDS_CHILDREN")
}

func TestLint_MinimumRequiredEqualsChildren_NoError(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Completion = game.CompletionMinimum
	doc.Structure.Children[0].Group.MinimumRequired = 1
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "MINIMUM_REQUIRED_EXCEEDS_CHILDREN", e.Code)
	}
}

// --- AUTO_ADVANCE_IGNORED ---

func TestLint_AutoAdvanceIgnored_Warning(t *testing.T) {
	autoTrue := true
	doc := validDoc()
	doc.Structure.Children[0].Group.Completion = game.CompletionAll
	doc.Structure.Children[0].Group.AutoAdvance = &autoTrue
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "AUTO_ADVANCE_IGNORED")
}

func TestLint_AutoAdvanceOnMinimumGroup_NoWarning(t *testing.T) {
	autoTrue := true
	doc := validDoc()
	doc.Structure.Children[0].Group.Completion = game.CompletionMinimum
	doc.Structure.Children[0].Group.MinimumRequired = 1
	doc.Structure.Children[0].Group.AutoAdvance = &autoTrue
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "AUTO_ADVANCE_IGNORED", w.Code)
	}
}

func TestLint_AutoAdvanceNil_NoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Completion = game.CompletionAll
	doc.Structure.Children[0].Group.AutoAdvance = nil
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "AUTO_ADVANCE_IGNORED", w.Code)
	}
}

func TestLint_ObjectiveDoc_MissingSlugAndTitle_Error(t *testing.T) {
	doc := validDoc()
	obj := doc.Structure.Children[0].Group.Children[0].Objective
	obj.Slug = ""
	obj.Title = ""
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "MISSING_SLUG")
	assert.Contains(t, codes, "MISSING_OBJECTIVE_TITLE")
}

func TestLint_ObjectiveProofContext_ContentOnly_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text"}}, // content-only, not interactive.
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "PROOF_CONTEXT_NO_INTERACTIVE_BLOCK")
}

func TestLint_ObjectiveProofContext_WithInteractiveBlock_NoError(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text"}, {"type": "quiz"}},
	}
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "PROOF_CONTEXT_NO_INTERACTIVE_BLOCK", e.Code)
	}
}

func TestLint_ObjectiveProofContext_Empty_NoError(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{}
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "PROOF_CONTEXT_NO_INTERACTIVE_BLOCK", e.Code)
	}
}

func TestLint_ObjectiveSlugDuplicate_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Objective: &game.ObjectiveDoc{Slug: "lobby", Title: "Lobby again"},
	})
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_DUPLICATE")
}

func TestLint_ObjectiveProofContext_InvalidBlockType_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		// "password" is interactive in the mock registry (satisfies
		// PROOF_CONTEXT_NO_INTERACTIVE_BLOCK, a layer-1 check that would otherwise
		// suppress layer 2 entirely) but absent from its contexts map, so it is
		// invalid everywhere, including ContextObjectiveProof.
		Blocks: []game.BlockDoc{{"type": "password"}},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "INVALID_CONTEXT")
}

func TestLint_ObjectiveBlockID_Duplicate_Error(t *testing.T) {
	doc := validDoc()
	obj := doc.Structure.Children[0].Group.Children[0].Objective
	obj.Proof = game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz", "id": "dup-1"}}}
	obj.Reveal = game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "text", "id": "dup-1"}}}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "BLOCK_ID_DUPLICATE")
}

func TestLint_ObjectiveBlockPoints_Disabled_Warning(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children[0].Group.Children[0].Objective.Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "points": float64(10)}},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "POINTS_DISABLED")
}

func TestLint_ObjectiveDepends_UndefinedVar_Warning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"nonexistent_var"}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

// TestLint_ObjectiveContextSets_DefinedAndUsed proves a context's own Sets field
// (fired when the whole context completes, not by any single block) reaches
// definedVars/usedVars the same way a block's sets does.
func TestLint_ObjectiveContextSets_DefinedAndUsed(t *testing.T) {
	doc := validDoc()
	obj := doc.Structure.Children[0].Group.Children[0].Objective
	obj.Proof.Sets = game.SetsField{"door_unlocked"}
	obj.Depends = game.DependsField{"door_unlocked"}
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_VAR", w.Code, w.Message)
		assert.NotEqual(t, "UNUSED_VAR", w.Code, w.Message)
	}
}

// TestLint_ObjectiveVarReference_UnknownSlug_Warning proves a typo'd
// objective.<slug> reference is caught: previously any non-empty suffix passed
// as a built-in, so a mistyped slug silently never matched at runtime.
func TestLint_ObjectiveVarReference_UnknownSlug_Warning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Objective.Depends = game.DependsField{"objective.does-not-exist"}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_OBJECTIVE_VAR")
}

func TestLint_ObjectiveVarReference_KnownSlug_NoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Objective: &game.ObjectiveDoc{
			Slug:    "unlock-door",
			Title:   "Unlock the door",
			Depends: game.DependsField{"objective.lobby"},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_OBJECTIVE_VAR", w.Code, w.Message)
	}
}
