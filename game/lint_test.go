package game_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRegistry implements game.BlockRegistry for testing.
type mockRegistry struct {
	validTypes  map[string]bool
	contexts    map[string][]game.BlockContext
	knownFields map[string][]string
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

func newTestRegistry() *mockRegistry {
	return &mockRegistry{
		validTypes: map[string]bool{
			"text":         true,
			"clue":         true,
			"quiz":         true,
			"start_button": true,
			"game_status":  true,
			"password":     true,
		},
		contexts: map[string][]game.BlockContext{
			"text":         {game.ContextLocationContent, game.ContextStart, game.ContextFinish},
			"clue":         {game.ContextNavigation},
			"quiz":         {game.ContextLocationContent},
			"start_button": {game.ContextStart},
			"game_status":  {game.ContextStart},
		},
	}
}

func validDoc() *game.GameDoc {
	return &game.GameDoc{
		Rapua: "v7",
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
