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

// boolPtr names a draft state. Draft is a pointer because omitting the key and
// writing false are different statements about an existing row.
func boolPtr(b bool) *bool { return &b }

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

// An empty routing falls to the same INVALID_ROUTING default as a bogus one:
// there is no "not set" state for a section with published children.
func TestLint_MissingRoutingOnSection(t *testing.T) {
	doc := validDoc()
	doc.Structure.Children[0].Routing = ""
	result := game.Lint(doc, newTestRegistry())
	require.True(t, result.HasError("INVALID_ROUTING"))
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

// Points on a quest with points switched off are left alone. The editor hides
// every points control while they are off, so a warning about one asked an
// author to fix something the page would not show them.
func TestLint_PointsWithPointsDisabled_SaysNothing(t *testing.T) {
	doc := validDoc()
	doc.Settings.EnablePoints = false
	leaf(doc).Reveal.Blocks = []game.BlockDoc{{"type": "text", "points": float64(10)}}

	result := game.Lint(doc, newTestRegistry())
	assert.Empty(t, result.Errors)
	assert.NotContains(t, warningCodes(result), "POINTS_DISABLED")
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

// Routing on a leaf says nothing: every objective is created with one, and an
// objective moved out of a section keeps what it had. The band and max_next
// below stay warnings because only an author can set those.
func TestLint_RoutingOnLeaf_IsNotWorthSaying(t *testing.T) {
	doc := validDoc()
	leaf(doc).Routing = game.RouteStrategyOrdered
	result := game.Lint(doc, newTestRegistry())
	assert.NotContains(t, warningCodes(result), "ROUTING_ON_LEAF")
	assert.Empty(t, result.Errors)
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

// Publishing must not be the moment a slug collision appears, so drafts take
// their slugs like anything else.
func TestLint_DraftSlugStillCollides(t *testing.T) {
	doc := validDoc()
	sec := section(doc)
	sec.Children = append(sec.Children, game.ObjectiveDoc{
		Slug:  leaf(doc).Slug,
		Title: "A parked copy",
		Draft: boolPtr(true),
	})

	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("SLUG_DUPLICATE"))
}

func TestLint_BandCountsPublishedChildrenOnly(t *testing.T) {
	doc := validDoc()
	sec := section(doc)
	sec.Children = []game.ObjectiveDoc{
		{Slug: "a", Title: "A"},
		{Slug: "b", Title: "B", Draft: boolPtr(true)},
	}
	sec.ChildrenMin = intPtr(2)

	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("BAND_OUT_OF_RANGE"),
		"a minimum of 2 over one published child can never be met")
}

// Drafting the root leaves the frontier with nothing to walk, so every player
// sees an empty quest and no screen says why.
func TestLint_RootDraft(t *testing.T) {
	doc := validDoc()
	doc.Structure.Draft = boolPtr(true)

	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("ROOT_DRAFT"))
}

// A filled band reaching zero because every child is drafted is not the author
// writing children_max: 0, and blaming that field sends them looking for
// something nobody set.
func TestLint_AllChildrenDraft(t *testing.T) {
	doc := validDoc()
	sec := section(doc)
	sec.Children = []game.ObjectiveDoc{{Slug: "parked", Title: "Parked", Draft: boolPtr(true)}}

	result := game.Lint(doc, newTestRegistry())
	assert.False(t, result.HasError("BAND_COMPLETES_AT_ZERO"),
		"nothing here says children_max is 0")
	assert.True(t, result.HasWarning("ALL_CHILDREN_DRAFT"))
}

func TestLint_SectionContentIsUntouchedByTheRootRule(t *testing.T) {
	doc := validDoc()
	sec := section(doc)
	sec.Proof.Blocks = []game.BlockDoc{{"type": "quiz"}}

	result := game.Lint(doc, newTestRegistry())
	assert.False(t, result.HasError("ROOT_HAS_CONTENT"))
}

// A field the model no longer has must be named, not swallowed. An import that
// accepts "color" and drops it tells an author their document worked, and the
// quest they get back is not the one they wrote.
func TestLint_NamesFieldsTheModelHasRetired(t *testing.T) {
	raw := []byte(`{
		"rapua": "v8",
		"name": "Retired fields",
		"settings": {},
		"start": [], "finish": [],
		"structure": {
			"slug": "root", "title": "Root", "routing": "free_roam",
			"color": "primary",
			"depends": ["something"],
			"proof": {"blocks": [], "sets": ["a-var"]},
			"reveal": {"blocks": []},
			"children": [
				{"slug": "one", "title": "One", "proof": {"blocks": []}, "reveal": {"blocks": []}}
			]
		}
	}`)

	result := game.LintJSON(raw, newTestRegistry())

	var named []string
	for _, diag := range append(result.Errors, result.Warnings...) {
		if diag.Code == "UNKNOWN_FIELD" {
			named = append(named, diag.Path)
		}
	}
	assert.Contains(t, named, "structure.color")
	assert.Contains(t, named, "structure.depends")
	assert.Contains(t, named, "structure.proof.sets")
}

// draft and description are current fields, and both were missing from the
// known set: a document using one was told the field did not exist. The two
// directions are one rule, so they are tested together.
func TestLint_AcceptsTheFieldsTheModelStillHas(t *testing.T) {
	raw := []byte(`{
		"rapua": "v8",
		"name": "Current fields",
		"settings": {},
		"start": [], "finish": [],
		"structure": {
			"slug": "root", "title": "Root", "routing": "ordered",
			"description": "Root detail",
			"proof": {"blocks": []}, "reveal": {"blocks": []},
			"children_min": 1, "children_max": 2, "max_next": 0,
			"finish_label": "Done",
			"children": [
				{"slug": "one", "title": "One", "draft": false,
				 "proof": {"blocks": []}, "reveal": {"blocks": []}}
			]
		}
	}`)

	for _, diag := range game.LintJSON(raw, newTestRegistry()).Warnings {
		assert.NotEqual(t, "UNKNOWN_FIELD", diag.Code, "unexpected: %s", diag.Path)
	}
}

// Block-level "sets" went with the variable system. A block still carrying one
// is naming a field nothing reads.
func TestLint_NamesBlockLevelSets(t *testing.T) {
	raw := []byte(`{
		"rapua": "v8", "name": "Sets", "settings": {},
		"start": [], "finish": [],
		"structure": {
			"slug": "root", "title": "Root",
			"proof": {"blocks": [{"type": "quiz", "sets": "answered"}]},
			"reveal": {"blocks": []}
		}
	}`)

	// The field check only runs for a type whose fields are known, so the mock
	// has to declare them the way the real registry does.
	registry := newTestRegistry()
	registry.knownFields = map[string][]string{"quiz": {"question", "options"}}

	result := game.LintJSON(raw, registry)

	var named []string
	for _, diag := range append(result.Errors, result.Warnings...) {
		if diag.Code == "UNKNOWN_FIELD" {
			named = append(named, diag.Path)
		}
	}
	assert.Equal(t, []string{"structure.proof.blocks[0].sets"}, named)
}
