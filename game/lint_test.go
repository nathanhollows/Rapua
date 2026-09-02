package game_test

import (
	"encoding/json"
	"fmt"
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

// validDoc returns a valid doc: a root with one section, holding one objective.
// Two levels rather than one, so tests can reach both a node with children and
// a leaf without building a tree each time.
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
		Structure: game.ObjectiveDoc{
			Slug:    "root",
			Title:   "Test Game",
			Routing: game.RouteStrategyFreeRoam,
			Children: []game.ObjectiveDoc{
				{
					Slug:    "stage-one",
					Title:   "Stage One",
					Color:   "primary",
					Routing: game.RouteStrategyFreeRoam,
					Children: []game.ObjectiveDoc{
						{
							Slug:  "lobby",
							Title: "The Lobby",
							Proof: game.ObjectiveContextDoc{
								Blocks: []game.BlockDoc{{"type": "quiz"}},
							},
						},
					},
				},
			},
		},
	}
}

// section is validDoc's mid-tree node: an objective with children.
func section(doc *game.GameDoc) *game.ObjectiveDoc {
	return &doc.Structure.Children[0]
}

// leaf is validDoc's childless objective, inside section.
func leaf(doc *game.GameDoc) *game.ObjectiveDoc {
	return &doc.Structure.Children[0].Children[0]
}

// intPtr names a band bound. Both bounds are pointers because an explicit 0
// means something an omitted bound does not.
func intPtr(n int) *int { return &n }

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
	leaf(doc).Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "nonexistent_block"}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "UNKNOWN_BLOCK_TYPE", result.Errors[0].Code)
}

// --- Layer 2: Semantic ---

func TestLint_DuplicateSlugs(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ObjectiveDoc{
		Slug:  "lobby", // duplicate.
		Title: "Another Lobby",
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
	leaf(doc).Reveal = game.ObjectiveContextDoc{
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

// --- Schema: block type checks ---

func TestLint_MissingBlockType(t *testing.T) {
	doc := validDoc()
	leaf(doc).Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{}}, // no "type" key.
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_InvalidBlockTypeNotString(t *testing.T) {
	doc := validDoc()
	leaf(doc).Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": 123}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_BLOCK_TYPE", result.Errors[0].Code)
}

func TestLint_NegativeBlockPoints(t *testing.T) {
	doc := validDoc()
	leaf(doc).Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "points": float64(-5)}},
	}
	result := game.Lint(doc, newTestRegistry())
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_POINTS", result.Errors[0].Code)
}

func TestLint_NegativeBlockPointsJsonNumber(t *testing.T) {
	doc := validDoc()
	leaf(doc).Reveal = game.ObjectiveContextDoc{
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
	leaf(doc).Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "bogus_field": "value"}},
	}
	result := game.Lint(doc, reg)
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "UNKNOWN_FIELD", result.Warnings[0].Code)
}

func TestLint_NilRegistry(t *testing.T) {
	doc := validDoc()
	leaf(doc).Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "any_type"}},
	}
	result := game.Lint(doc, nil)
	assert.Empty(t, result.Errors)
}

// --- Semantic: group slug deduplication, block ID duplicates ---

func TestLint_BlockIDDuplicate(t *testing.T) {
	doc := validDoc()
	leaf(doc).Reveal = game.ObjectiveContextDoc{
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

func TestLint_PointsDisabledJsonNumber(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	leaf(doc).Reveal = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text", "points": json.Number("10")}},
	}
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "POINTS_DISABLED", result.Warnings[0].Code)
}

// --- When / variable resolution ---

func TestLint_UndefinedVar_ObjectiveDepends(t *testing.T) {
	doc := validDoc()
	leaf(doc).Depends = game.DependsField{"ghost_var"}
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
	leaf(doc).Depends = game.DependsField{"not ghost_var"}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_VAR")
}

func TestLint_DependsEmptyName_Error(t *testing.T) {
	doc := validDoc()
	leaf(doc).Depends = game.DependsField{"   "}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_EMPTY_NAME"))
}

func TestLint_DependsSelfReference_Error(t *testing.T) {
	doc := validDoc()
	leaf(doc).Depends = game.DependsField{"objective.lobby"}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_CYCLE"))
}

func TestLint_DependsMutualCycle_Error(t *testing.T) {
	doc := validDoc()
	leaf(doc).Depends = game.DependsField{"objective.annex"}
	section(doc).Children = append(
		section(doc).Children,
		game.ObjectiveDoc{
			Slug:    "annex",
			Title:   "The Annex",
			Depends: game.DependsField{"objective.lobby"},
		},
	)
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_CYCLE"))
}

// Negation does not break a cycle: "not other" still cannot be evaluated until
// other has been, so the chain is just as unreachable.
func TestLint_DependsNegatedCycle_Error(t *testing.T) {
	doc := validDoc()
	leaf(doc).Depends = game.DependsField{"not objective.lobby"}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_CYCLE"))
}

// A diamond is not a cycle: two objectives may both gate on a third.
func TestLint_DependsDiamond_NoError(t *testing.T) {
	doc := validDoc()
	for _, slug := range []string{"left", "right"} {
		section(doc).Children = append(
			section(doc).Children,
			game.ObjectiveDoc{
				Slug:    slug,
				Title:   slug,
				Depends: game.DependsField{"objective.lobby"},
			},
		)
	}
	result := game.Lint(doc, newTestRegistry())
	assert.False(t, result.HasError("DEPENDS_CYCLE"))
}

func TestLint_DefinedVar_NoWarning(t *testing.T) {
	doc := validDoc()
	// Block sets "score", a sibling objective's depends references it: clean.
	leaf(doc).Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{
			{
				"type": "quiz",
				"sets": []any{"score"},
			},
		},
	}
	leaf(doc).Depends = game.DependsField{"score"}
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_VAR", w.Code)
	}
}

func TestLint_UnusedVar(t *testing.T) {
	doc := validDoc()
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "quiz", "sets": []any{"score"}}},
	}
	// A second objective's depends references "score": should suppress UNUSED_VAR.
	section(doc).Children = append(section(doc).Children, game.ObjectiveDoc{
		Slug:    "loc2",
		Title:   "Location 2",
		Depends: game.DependsField{"score"},
	})
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNUSED_VAR", w.Code)
	}
}

// --- Non-interactive block sets tests ---

func TestLint_SetsOnNonInteractiveBlock_Warning(t *testing.T) {
	doc := validDoc()
	leaf(doc).Reveal = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
			leaf(doc).Reveal = game.ObjectiveContextDoc{
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
	leaf(doc).Slug = "The-Lobby"
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_INVALID_FORMAT")
}

func TestLint_SlugInvalidFormat_LeadingHyphen(t *testing.T) {
	doc := validDoc()
	leaf(doc).Slug = "-lobby"
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_INVALID_FORMAT")
}

func TestLint_SlugValidFormat_NoError(t *testing.T) {
	doc := validDoc()
	leaf(doc).Slug = "the-lobby-2"
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "SLUG_INVALID_FORMAT", e.Code)
	}
}

// --- MINIMUM_REQUIRED_EXCEEDS_CHILDREN ---

// --- AUTO_ADVANCE_IGNORED ---

func TestLint_ObjectiveDoc_MissingSlugAndTitle_Error(t *testing.T) {
	doc := validDoc()
	obj := leaf(doc)
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
		Blocks: []game.BlockDoc{{"type": "text"}, {"type": "quiz"}},
	}
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "PROOF_CONTEXT_NO_INTERACTIVE_BLOCK", e.Code)
	}
}

func TestLint_ObjectiveProofContext_Empty_NoError(t *testing.T) {
	doc := validDoc()
	leaf(doc).Proof = game.ObjectiveContextDoc{}
	result := game.Lint(doc, newTestRegistry())
	for _, e := range result.Errors {
		assert.NotEqual(t, "PROOF_CONTEXT_NO_INTERACTIVE_BLOCK", e.Code)
	}
}

func TestLint_ObjectiveSlugDuplicate_Error(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ObjectiveDoc{Slug: "lobby", Title: "Lobby again"})
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	assert.Contains(t, codes, "SLUG_DUPLICATE")
}

func TestLint_ObjectiveProofContext_InvalidBlockType_Error(t *testing.T) {
	doc := validDoc()
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	obj := leaf(doc)
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
	leaf(doc).Proof = game.ObjectiveContextDoc{
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
	leaf(doc).Depends = game.DependsField{"nonexistent_var"}
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
	obj := leaf(doc)
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
	leaf(doc).Depends = game.DependsField{"objective.does-not-exist"}
	result := game.Lint(doc, newTestRegistry())
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	assert.Contains(t, codes, "UNDEFINED_OBJECTIVE_VAR")
}

func TestLint_ObjectiveVarReference_KnownSlug_NoWarning(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children = append(doc.Structure.Children, game.ObjectiveDoc{
		Slug:    "unlock-door",
		Title:   "Unlock the door",
		Depends: game.DependsField{"objective.lobby"},
	})
	result := game.Lint(doc, newTestRegistry())
	for _, w := range result.Warnings {
		assert.NotEqual(t, "UNDEFINED_OBJECTIVE_VAR", w.Code, w.Message)
	}
}

// --- Completion band ---

func TestFillBand(t *testing.T) {
	tests := []struct {
		name        string
		minChildren *int
		maxChildren *int
		childCount  int
		want        game.Band
	}{
		{
			name:       "both omitted requires every child",
			childCount: 3, want: game.Band{Min: 3, Max: 3},
		},
		{
			name:        "min and max equal auto-completes at that count",
			minChildren: intPtr(1), maxChildren: intPtr(1), childCount: 3,
			want: game.Band{Min: 1, Max: 1},
		},
		{
			name:        "min only widens max to the child count",
			minChildren: intPtr(5), childCount: 12,
			want: game.Band{Min: 5, Max: 12},
		},
		{
			name:        "max only widens min to zero",
			maxChildren: intPtr(2), childCount: 6,
			want: game.Band{Min: 0, Max: 2},
		},
		{
			name:        "an explicit zero min is not an omitted min",
			minChildren: intPtr(0), childCount: 4,
			want: game.Band{Min: 0, Max: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, game.FillBand(tt.minChildren, tt.maxChildren, tt.childCount))
		})
	}
}

func TestBand_AutoCompletes(t *testing.T) {
	assert.True(t, game.Band{Min: 2, Max: 2}.AutoCompletes(), "no range means no decision to make")
	assert.False(t, game.Band{Min: 1, Max: 3}.AutoCompletes(), "a range waits on the player")
}

func TestLint_BandMinExceedsMax_Error(t *testing.T) {
	doc := validDoc()
	section(doc).ChildrenMin = intPtr(1)
	section(doc).ChildrenMax = intPtr(0)
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("BAND_MIN_EXCEEDS_MAX"))
}

func TestLint_BandOutOfRange_Error(t *testing.T) {
	for _, tt := range []struct {
		name        string
		minChildren *int
		maxChildren *int
	}{
		{name: "min exceeds child count", minChildren: intPtr(2)},
		{name: "max exceeds child count", maxChildren: intPtr(9)},
		{name: "negative min", minChildren: intPtr(-1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDoc()
			section(doc).ChildrenMin = tt.minChildren
			section(doc).ChildrenMax = tt.maxChildren
			result := game.Lint(doc, newTestRegistry())
			assert.True(t, result.HasError("BAND_OUT_OF_RANGE"))
		})
	}
}

// A band exactly at the child count is the ordinary "all of them" case.
func TestLint_BandAtChildCount_NoError(t *testing.T) {
	doc := validDoc()
	section(doc).ChildrenMin = intPtr(1)
	section(doc).ChildrenMax = intPtr(1)
	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
}

func TestLint_BandOnLeaf_Warning(t *testing.T) {
	doc := validDoc()
	leaf(doc).ChildrenMin = intPtr(0)
	result := game.Lint(doc, newTestRegistry())
	assert.Contains(t, warningCodes(result), "BAND_ON_LEAF")
	assert.Empty(t, result.Errors, "an inert field is not an error")
}

func TestLint_RoutingOnLeaf_Warning(t *testing.T) {
	doc := validDoc()
	leaf(doc).Routing = game.RouteStrategyOrdered
	assert.Contains(t, warningCodes(game.Lint(doc, newTestRegistry())), "ROUTING_ON_LEAF")
}

// The finish button only appears on a node in a range, so a label anywhere else
// is a promise the UI never keeps.
func TestLint_FinishLabelOnAutoCompletingNode_Warning(t *testing.T) {
	doc := validDoc()
	section(doc).FinishLabel = "Done here"
	assert.Contains(t, warningCodes(game.Lint(doc, newTestRegistry())), "FINISH_LABEL_UNREACHABLE")
}

func TestLint_FinishLabelOnRangedNode_NoWarning(t *testing.T) {
	doc := validDoc()
	section(doc).Children = append(section(doc).Children, game.ObjectiveDoc{Slug: "annex", Title: "Annex"})
	section(doc).ChildrenMin = intPtr(1)
	section(doc).ChildrenMax = intPtr(2)
	section(doc).FinishLabel = "Done here"
	assert.NotContains(t, warningCodes(game.Lint(doc, newTestRegistry())), "FINISH_LABEL_UNREACHABLE")
}

func TestLint_MaxNextIgnoredWithoutRandomisedRouting_Warning(t *testing.T) {
	doc := validDoc()
	section(doc).MaxNext = 2
	assert.Contains(t, warningCodes(game.Lint(doc, newTestRegistry())), "MAX_NEXT_IGNORED")
}

// --- Nesting depth ---

func TestLint_NestingTooDeep_Warning(t *testing.T) {
	doc := validDoc()
	// validDoc is already root -> section -> leaf, so growing the leaf downwards
	// pushes past the cap.
	node := leaf(doc)
	for i := range 4 {
		node.Routing = game.RouteStrategyFreeRoam
		node.Children = []game.ObjectiveDoc{{
			Slug:  fmt.Sprintf("deep-%d", i),
			Title: fmt.Sprintf("Deep %d", i),
		}}
		node = &node.Children[0]
	}
	assert.Contains(t, warningCodes(game.Lint(doc, newTestRegistry())), "NESTING_TOO_DEEP")
}

func TestLint_ShallowNesting_NoWarning(t *testing.T) {
	doc := validDoc()
	assert.NotContains(t, warningCodes(game.Lint(doc, newTestRegistry())), "NESTING_TOO_DEEP")
}

// warningCodes collects the codes of every warning, for the many assertions
// that care which rules fired rather than what they said.
func warningCodes(result game.LintResult) []string {
	codes := make([]string, len(result.Warnings))
	for i, w := range result.Warnings {
		codes[i] = w.Code
	}
	return codes
}

// The secret routing strategy is retired. A document still naming it should be
// told how reachability works now, not just that the value is invalid.
func TestLint_SecretRouting_ErrorNamesTheReplacement(t *testing.T) {
	doc := validDoc()
	section(doc).Routing = "secret"
	result := game.Lint(doc, newTestRegistry())

	require.True(t, result.HasError("INVALID_ROUTING"))
	for _, e := range result.Errors {
		if e.Code == "INVALID_ROUTING" {
			assert.Contains(t, e.Message, "retired")
			assert.Contains(t, e.Message, "scan")
		}
	}
}

// children_max of 0 completes the objective before the player can reach any
// child, closing the subtree. The neighbouring max_next does read 0 as "all of
// them", so the mistake is an easy one to make.
func TestLint_BandCompletesAtZero_Error(t *testing.T) {
	doc := validDoc()
	section(doc).ChildrenMax = intPtr(0)
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("BAND_COMPLETES_AT_ZERO"))
}

// An objective with no children has nothing to complete early, so the rule does
// not fire there: BAND_ON_LEAF already covers it.
func TestLint_BandCompletesAtZero_NotOnLeaf(t *testing.T) {
	doc := validDoc()
	leaf(doc).ChildrenMax = intPtr(0)
	result := game.Lint(doc, newTestRegistry())
	assert.False(t, result.HasError("BAND_COMPLETES_AT_ZERO"))
}

// A section is an objective row like any other, so its own proof and reveal
// blocks are stored and read back rather than being rejected.
func TestLint_SectionWithOwnBlocks_NoError(t *testing.T) {
	doc := validDoc()
	sec := section(doc)
	sec.Proof.Blocks = []game.BlockDoc{{"type": "quiz"}}
	sec.Reveal.Blocks = []game.BlockDoc{{"type": "text"}}

	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
}
