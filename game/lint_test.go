package game_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRegistry implements game.BlockRegistry for testing.
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
			"text":         {game.ContextLocationContent, game.ContextStart, game.ContextFinish},
			"clue":         {game.ContextNavigation},
			"quiz":         {game.ContextLocationContent},
			"choice":       {game.ContextLocationContent},
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
						{Location: &game.LocationDoc{
							Slug:       "lobby",
							Name:       "The Lobby",
							Content:    []game.BlockDoc{{"type": "text"}},
							Navigation: []game.BlockDoc{{"type": "clue"}},
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

func TestLint_MissingLocationSlug(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Slug = ""
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_SLUG", result.Errors[0].Code)
}

func TestLint_MissingLocationName(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Name = ""
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_LOCATION_NAME", result.Errors[0].Code)
}

func TestLint_UnknownBlockType(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "nonexistent_block"},
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
		Location: &game.LocationDoc{
			Slug:       "lobby", // duplicate
			Name:       "Another Lobby",
			Content:    []game.BlockDoc{},
			Navigation: []game.BlockDoc{},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "SLUG_DUPLICATE", result.Errors[0].Code)
}

func TestLint_InvalidContext(t *testing.T) {
	doc := validDoc()
	// quiz can't be in start context
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
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "points": float64(10)},
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

// --- Schema: finish blocks, group name, empty child, location checks ---

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

func TestLint_NegativeLocationPoints(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Points = -1
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_POINTS", result.Errors[0].Code)
}

func TestLint_ZeroCoordinatesWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Marker = &game.MarkerDoc{Lat: 0, Lng: 0}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "ZERO_COORDINATES", result.Warnings[0].Code)
}

func TestLint_NonZeroCoordinatesNoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Marker = &game.MarkerDoc{Lat: 51.5, Lng: -0.1}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
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
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{{}} // no "type" key
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_InvalidBlockTypeNotString(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{{"type": 123}}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_NegativeBlockPoints(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "points": float64(-5)},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_POINTS", result.Errors[0].Code)
}

func TestLint_NegativeBlockPointsJsonNumber(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "points": json.Number("-5")},
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
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "bogus_field": "value"},
	}
	result := game.Lint(doc, reg)
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "UNKNOWN_FIELD", result.Warnings[0].Code)
}

func TestLint_NilRegistry(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{{"type": "any_type"}}
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
				{Location: &game.LocationDoc{
					Slug:    "lobby", // duplicate of top-level location
					Name:    "Lobby Copy",
					Content: []game.BlockDoc{},
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
						{Location: &game.LocationDoc{
							Slug:    "lobby", // duplicate of top-level location
							Name:    "Deep Lobby",
							Content: []game.BlockDoc{},
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
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "id": "block-abc"},
		{"type": "text", "id": "block-abc"},
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

func TestLint_PointsDisabledInGroup(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Group: &game.GroupDoc{
			Name:       "Group",
			Color:      "primary",
			Routing:    game.RouteStrategyFreeRoam,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Location: &game.LocationDoc{
					Slug:       "station",
					Name:       "Station",
					Points:     10,
					Content:    []game.BlockDoc{{"type": "text"}},
					Navigation: []game.BlockDoc{{"type": "clue"}},
				}},
			},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

func TestLint_PointsDisabledLocationPoints(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children[0].Group.Children[0].Location.Points = 5
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

func TestLint_PointsDisabledJsonNumber(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "points": json.Number("10")},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

func TestLint_InvalidNavigationContext(t *testing.T) {
	doc := validDoc()
	// quiz is not registered for ContextNavigation — should produce INVALID_CONTEXT
	doc.Structure.Children[0].Group.Children[0].Location.Navigation = []game.BlockDoc{{"type": "quiz"}}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_CONTEXT", result.Errors[0].Code)
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
						{Location: &game.LocationDoc{
							Slug:       "deep-station",
							Name:       "Deep Station",
							Points:     10,
							Content:    []game.BlockDoc{{"type": "text"}},
							Navigation: []game.BlockDoc{{"type": "clue"}},
						}},
					},
				}},
			},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	// POINTS_DISABLED for deep-station.points
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

func TestLint_RootLocationHidden(t *testing.T) {
	// Locations placed directly under structure.children (not inside a group) are never shown.
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ChildDoc{
		Location: &game.LocationDoc{
			Slug:       "orphan",
			Name:       "Orphan Stop",
			Content:    []game.BlockDoc{{"type": "text"}},
			Navigation: []game.BlockDoc{{"type": "clue"}},
		},
	})
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "ROOT_LOCATION_HIDDEN", result.Warnings[0].Code)
}

func TestLint_RootHasNoGroups(t *testing.T) {
	// Structure with only root-level locations and no groups at all — all locations hidden.
	doc := validDoc()
	doc.Structure.Children = []game.ChildDoc{
		{Location: &game.LocationDoc{
			Slug:       "stop-a",
			Name:       "Stop A",
			Content:    []game.BlockDoc{{"type": "text"}},
			Navigation: []game.BlockDoc{{"type": "clue"}},
		}},
		{Location: &game.LocationDoc{
			Slug:       "stop-b",
			Name:       "Stop B",
			Content:    []game.BlockDoc{{"type": "text"}},
			Navigation: []game.BlockDoc{{"type": "clue"}},
		}},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	// One ROOT_LOCATION_HIDDEN per bare location
	require.Len(t, result.Warnings, 2)
	assert.Equal(t, "ROOT_LOCATION_HIDDEN", result.Warnings[0].Code)
	assert.Equal(t, "ROOT_LOCATION_HIDDEN", result.Warnings[1].Code)
}

func TestLint_NoNavigationBlocksWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Navigation = []game.BlockDoc{}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "NO_NAVIGATION_BLOCKS", result.Warnings[0].Code)
}

func TestLint_NoContentBlocksWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "NO_CONTENT_BLOCKS", result.Warnings[0].Code)
}

// --- When / variable resolution ---

func TestLint_UndefinedVar_LocationWhen(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.When = &game.WhenClause{
		AllOf: []game.Condition{{Var: "ghost_var"}},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

func TestLint_UndefinedVar_GroupWhen(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.When = &game.WhenClause{
		AnyOf: []game.Condition{{Var: "ghost_var"}},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

func TestLint_UndefinedVar_BlockWhen(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{
			"type": "quiz",
			"when": map[string]any{
				"all_of": []any{
					map[string]any{"var": "ghost_var"},
				},
			},
		},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

func TestLint_DefinedVar_NoWarning(t *testing.T) {
	doc := validDoc()
	// Block sets "score", location when references "score" — should be clean
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{
			"type": "quiz",
			"sets": map[string]any{"score": "true"},
		},
	}
	doc.Structure.Children[0].Group.Children[0].Location.When = &game.WhenClause{
		AllOf: []game.Condition{{Var: "score"}},
	}
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_VAR", w.Code)
	}
}

func TestLint_WhenUnreachableVar_Min1AutoAdvance(t *testing.T) {
	autoAdvanceTrue := true
	doc := validDoc()
	// Group with completion=minimum, min=1, auto_advance=true
	doc.Structure.Children[0].Group.Completion = game.CompletionMinimum
	doc.Structure.Children[0].Group.MinimumRequired = 1
	doc.Structure.Children[0].Group.AutoAdvance = &autoAdvanceTrue

	loc := doc.Structure.Children[0].Group.Children[0].Location
	// First location sets "visited_loc1"
	loc.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"visited_loc1": "true"}},
	}
	// Add a second location that depends on "visited_loc1" being set
	secondLoc := &game.LocationDoc{
		Slug: "loc2",
		Name: "Location 2",
		When: &game.WhenClause{
			AllOf: []game.Condition{{Var: "visited_loc1"}},
		},
		Content:    []game.BlockDoc{{"type": "text"}},
		Navigation: []game.BlockDoc{{"type": "clue"}},
	}
	doc.Structure.Children[0].Group.Children = append(
		doc.Structure.Children[0].Group.Children,
		game.ChildDoc{Location: secondLoc},
	)

	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "WHEN_UNREACHABLE_VAR")
}

func TestLint_WhenUnreachableVar_NotMin1_NoWarning(t *testing.T) {
	doc := validDoc()
	// Group with completion=minimum, min=2 — should NOT warn
	doc.Structure.Children[0].Group.Completion = game.CompletionMinimum
	doc.Structure.Children[0].Group.MinimumRequired = 2
	loc := doc.Structure.Children[0].Group.Children[0].Location
	loc.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"visited_loc1": "true"}},
	}
	secondLoc := &game.LocationDoc{
		Slug: "loc2",
		Name: "Location 2",
		When: &game.WhenClause{
			AllOf: []game.Condition{{Var: "visited_loc1"}},
		},
		Content:    []game.BlockDoc{{"type": "text"}},
		Navigation: []game.BlockDoc{{"type": "clue"}},
	}
	doc.Structure.Children[0].Group.Children = append(
		doc.Structure.Children[0].Group.Children,
		game.ChildDoc{Location: secondLoc},
	)
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "WHEN_UNREACHABLE_VAR", w.Code)
	}
}

func TestLint_UnusedVar(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"score": "true"}},
	}
	// "score" is set but no when clause references it
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNUSED_VAR")
}

func TestLint_UnusedVar_UsedElsewhere_NoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"score": "true"}},
	}
	// A second location's when references "score" — should suppress UNUSED_VAR
	secondLoc := &game.LocationDoc{
		Slug: "loc2",
		Name: "Location 2",
		When: &game.WhenClause{
			AllOf: []game.Condition{{Var: "score"}},
		},
		Content:    []game.BlockDoc{{"type": "text"}},
		Navigation: []game.BlockDoc{{"type": "clue"}},
	}
	doc.Structure.Children[0].Group.Children = append(
		doc.Structure.Children[0].Group.Children,
		game.ChildDoc{Location: secondLoc},
	)
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNUSED_VAR", w.Code)
	}
}

// --- Non-interactive block when/sets tests ---

func TestLint_WhenOnNonInteractiveBlock_UndefinedVar(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{
			"type": "text",
			"when": map[string]any{
				"all_of": []any{
					map[string]any{"var": "ghost_var"},
				},
			},
		},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

func TestLint_WhenOnNonInteractiveBlock_ValidVar_NoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"score": "true"}},
		{
			"type": "text",
			"when": map[string]any{
				"all_of": []any{
					map[string]any{"var": "score"},
				},
			},
		},
	}
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_VAR", w.Code)
	}
}

func TestLint_SetsOnNonInteractiveBlock_Warning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "text", "sets": map[string]any{"foo": "true"}},
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
func TestLint_SetsAsList_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": []any{"found_clue"}},
	}
	result := game.Lint(doc, newTestRegistry())

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	require.Contains(t, codes, "SETS_NOT_OBJECT")

	for _, e := range result.Errors {
		if e.Code == "SETS_NOT_OBJECT" {
			assert.Contains(t, e.Path, ".sets")
			assert.Contains(t, e.Message, "must be an object")
		}
	}
}

func TestLint_SetsAsScalar_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": "found_clue"},
	}
	result := game.Lint(doc, newTestRegistry())

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SETS_NOT_OBJECT")
}

func TestLint_SetsReservedNamespace_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"objective.find-maisie": "done"}},
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
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "choice", "options": []any{
			map[string]any{"label": "Yes", "sets": "objective.find-maisie"},
		}},
	}
	result := game.Lint(doc, reg)

	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	require.Contains(t, codes, "SETS_RESERVED_NAMESPACE")

	for _, e := range result.Errors {
		if e.Code == "SETS_RESERVED_NAMESPACE" {
			assert.Contains(t, e.Path, "content[0]") // block path, not .sets
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
			block: game.BlockDoc{"type": "not_a_block", "sets": []any{"found_clue"}},
			want:  "UNKNOWN_BLOCK_TYPE",
		},
		{
			name:  "missing type",
			block: game.BlockDoc{"sets": []any{"found_clue"}},
			want:  "MISSING_BLOCK_TYPE",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDoc()
			doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{tt.block}
			result := game.Lint(doc, newTestRegistry())

			codes := make([]string, len(result.Errors))
			for i, e := range result.Errors {
				codes[i] = e.Code
			}
			assert.Contains(t, codes, tt.want)
			assert.Contains(t, codes, "SETS_NOT_OBJECT")
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

func TestLintJSON_UnknownFieldInLocation(t *testing.T) {
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
						"location": {
							"slug": "loc-a",
							"name": "Loc A",
							"content": [],
							"navigation": [],
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

func TestLint_WhenUnreachableVar_AutoAdvanceFalse_NoWarning(t *testing.T) {
	autoAdvanceFalse := false
	doc := validDoc()
	doc.Structure.Children[0].Group.Completion = game.CompletionMinimum
	doc.Structure.Children[0].Group.MinimumRequired = 1
	doc.Structure.Children[0].Group.AutoAdvance = &autoAdvanceFalse

	loc := doc.Structure.Children[0].Group.Children[0].Location
	loc.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"visited_loc1": "true"}},
	}
	secondLoc := &game.LocationDoc{
		Slug: "loc2",
		Name: "Location 2",
		When: &game.WhenClause{
			AllOf: []game.Condition{{Var: "visited_loc1"}},
		},
		Content:    []game.BlockDoc{{"type": "text"}},
		Navigation: []game.BlockDoc{{"type": "clue"}},
	}
	doc.Structure.Children[0].Group.Children = append(
		doc.Structure.Children[0].Group.Children,
		game.ChildDoc{Location: secondLoc},
	)
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "WHEN_UNREACHABLE_VAR", w.Code)
	}
}

// --- SLUG_INVALID_FORMAT ---

func TestLint_SlugInvalidFormat_Uppercase(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Slug = "The-Lobby"
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_INVALID_FORMAT")
}

func TestLint_SlugInvalidFormat_LeadingHyphen(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Slug = "-lobby"
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_INVALID_FORMAT")
}

func TestLint_SlugValidFormat_NoError(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Slug = "the-lobby-2"
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
	// Group has 1 child
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
	// Structure has 1 child
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

// --- WHEN_VACUOUS ---

func TestLint_WhenVacuous_EmptyAllOfAnyOf(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.When = &game.WhenClause{}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "WHEN_VACUOUS")
}

func TestLint_WhenWithConditions_NotVacuous(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Group.Children[0].Location.Content = []game.BlockDoc{
		{"type": "quiz", "sets": map[string]any{"score": "true"}},
	}
	doc.Structure.Children[0].Group.Children[0].Location.When = &game.WhenClause{
		AllOf: []game.Condition{{Var: "score"}},
	}
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "WHEN_VACUOUS", w.Code)
	}
}

// --- WHEN_ON_START_BLOCK ---

func TestLint_WhenOnStartBlock_Warning(t *testing.T) {
	doc := validDoc()
	doc.Start = []game.BlockDoc{
		{"type": "start_button"},
		{
			"type": "text",
			"when": map[string]any{"all_of": []any{map[string]any{"var": "score"}}},
		},
	}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "WHEN_ON_START_BLOCK")
}

func TestLint_WhenOnStartBlock_NoWhenClause_NoWarning(t *testing.T) {
	doc := validDoc()
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "WHEN_ON_START_BLOCK", w.Code)
	}
}
