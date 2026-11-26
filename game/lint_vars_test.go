package game_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

// varDoc is a quest where one objective's quiz opens another: the smallest
// working use of a variable.
func varDoc() *game.GameDoc {
	return &game.GameDoc{
		Rapua:  "v8",
		Name:   "Vars",
		Start:  []game.BlockDoc{{"type": "start_button"}},
		Finish: []game.BlockDoc{},
		Structure: game.ObjectiveDoc{
			Slug:    "root",
			Title:   "Root",
			Routing: game.RouteStrategyFreeRoam,
			Children: []game.ObjectiveDoc{
				{
					Slug:  "gate",
					Title: "Gate",
					Proof: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz", "sets": "key"}}},
				},
				{
					Slug:    "door",
					Title:   "Door",
					Depends: game.DependsField{"key"},
					Proof:   game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz"}}},
				},
			},
		},
	}
}

func gate(doc *game.GameDoc) *game.ObjectiveDoc { return &doc.Structure.Children[0] }
func door(doc *game.GameDoc) *game.ObjectiveDoc { return &doc.Structure.Children[1] }

func choice(multi bool, options ...string) game.BlockDoc {
	opts := make([]any, 0, len(options))
	for _, o := range options {
		opts = append(opts, map[string]any{"label": o, "sets": o})
	}
	return game.BlockDoc{"type": "choice", "multi_select": multi, "options": opts}
}

func lintCodes(doc *game.GameDoc) ([]string, []string) {
	result := game.Lint(doc, newTestRegistry())
	var errs []string
	for _, e := range result.Errors {
		errs = append(errs, e.Code)
	}
	return errs, warningCodes(result)
}

func TestLintVars_WorkingGateIsClean(t *testing.T) {
	errs, warns := lintCodes(varDoc())
	assert.Empty(t, errs)
	assert.Empty(t, warns)
}

func TestLintVars_BlockShape(t *testing.T) {
	cases := []struct {
		name  string
		block game.BlockDoc
		code  string
	}{
		{"sets must be one name", game.BlockDoc{"type": "quiz", "sets": []any{"a"}}, "SETS_NOT_STRING"},
		{"content cannot set", game.BlockDoc{"type": "text", "sets": "a"}, "SETS_ON_STATIC_BLOCK"},
		{"a choice sets per option", game.BlockDoc{"type": "choice", "sets": "a"}, "SETS_ON_CHOICE"},
		{"bad set name", game.BlockDoc{"type": "quiz", "sets": "Big-Name"}, "VAR_NAME_INVALID"},
		{"completed elsewhere", game.BlockDoc{"type": "team_name", "sets": "a"}, "SETS_UNSUPPORTED"},
		{"answers cannot wait", game.BlockDoc{"type": "quiz", "depends": []any{"key"}}, "DEPENDS_ON_INTERACTIVE_BLOCK"},
		{"depends must be a list", game.BlockDoc{"type": "text", "depends": "key"}, "DEPENDS_NOT_LIST"},
		{"depends of strings", game.BlockDoc{"type": "text", "depends": []any{1.0}}, "DEPENDS_NOT_LIST"},
		{"empty depends name", game.BlockDoc{"type": "text", "depends": []any{""}}, "DEPENDS_EMPTY_NAME"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := varDoc()
			gate(doc).Proof.Blocks = append(gate(doc).Proof.Blocks, c.block)
			registry := newTestRegistry()
			registry.validTypes["team_name"] = true
			registry.interactive["team_name"] = true
			var errs []string
			for _, e := range game.Lint(doc, registry).Errors {
				errs = append(errs, e.Code)
			}
			assert.Contains(t, errs, c.code)
		})
	}
}

func TestLintVars_ObjectiveDependsNames(t *testing.T) {
	doc := varDoc()
	door(doc).Depends = game.DependsField{"key", "Key", "", "key"}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("VAR_NAME_INVALID"))
	assert.True(t, result.HasError("DEPENDS_EMPTY_NAME"))
	assert.True(t, result.HasWarning("DEPENDS_DUPLICATE"))
}

func TestLintVars_RootCannotWait(t *testing.T) {
	doc := varDoc()
	doc.Structure.Depends = game.DependsField{"key"}
	errs, _ := lintCodes(doc)
	assert.Contains(t, errs, "ROOT_DEPENDS")
}

func TestLintVars_UndefinedVar(t *testing.T) {
	doc := varDoc()
	door(doc).Depends = game.DependsField{"key", "nobody_sets_this"}
	errs, _ := lintCodes(doc)
	assert.Equal(t, []string{"UNDEFINED_VAR"}, errs, "named once, not again as unsatisfiable")
}

func TestLintVars_OnlySetInDraft(t *testing.T) {
	doc := varDoc()
	gate(doc).Draft = boolPtr(true)
	errs, warns := lintCodes(doc)
	assert.Empty(t, errs)
	assert.Contains(t, warns, "VAR_ONLY_SET_IN_DRAFT")

	door(doc).Draft = boolPtr(true)
	_, warns = lintCodes(doc)
	assert.NotContains(t, warns, "VAR_ONLY_SET_IN_DRAFT", "a draft waiting on a draft publishes with it")
}

func TestLintVars_UnusedVar(t *testing.T) {
	doc := varDoc()
	door(doc).Depends = nil
	_, warns := lintCodes(doc)
	assert.Contains(t, warns, "UNUSED_VAR")

	doc = varDoc()
	gate(doc).Proof.Blocks = append(gate(doc).Proof.Blocks, choice(false, "left", "right"))
	_, warns = lintCodes(doc)
	assert.NotContains(t, warns, "UNUSED_VAR", "a choice's variables also name its options")
}

func TestLintVars_GateBehindItself(t *testing.T) {
	doc := varDoc()
	// The door's own proof sets the key it waits on.
	gate(doc).Proof.Blocks = []game.BlockDoc{{"type": "quiz"}}
	door(doc).Proof.Blocks = []game.BlockDoc{{"type": "quiz", "sets": "key"}}
	errs, _ := lintCodes(doc)
	assert.Equal(t, []string{"DEPENDS_UNSATISFIABLE"}, errs)
}

func TestLintVars_GateSetBelowItself(t *testing.T) {
	doc := varDoc()
	door(doc).Proof.Blocks = nil
	door(doc).Routing = game.RouteStrategyFreeRoam
	door(doc).Children = []game.ObjectiveDoc{{
		Slug:  "inside",
		Title: "Inside",
		Proof: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz", "sets": "key"}}},
	}}
	gate(doc).Proof.Blocks = []game.BlockDoc{{"type": "quiz"}}
	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("DEPENDS_UNSATISFIABLE"))
	assert.Len(t, result.Errors, 1, "the shut child is not blamed as well")
}

func TestLintVars_CycleAcrossObjectives(t *testing.T) {
	doc := varDoc()
	gate(doc).Depends = game.DependsField{"door_done"}
	door(doc).Proof.Blocks = []game.BlockDoc{{"type": "quiz", "sets": "door_done"}}
	errs, _ := lintCodes(doc)
	assert.Equal(t, []string{"DEPENDS_UNSATISFIABLE", "DEPENDS_UNSATISFIABLE"}, errs)
}

func TestLintVars_OrderedSiblingDeadlock(t *testing.T) {
	doc := varDoc()
	doc.Structure.Routing = game.RouteStrategyOrdered
	// The door comes first in order but needs what the gate, after it, sets.
	doc.Structure.Children[0], doc.Structure.Children[1] = doc.Structure.Children[1], doc.Structure.Children[0]
	errs, _ := lintCodes(doc)
	assert.Equal(t, []string{"DEPENDS_UNSATISFIABLE"}, errs)

	doc = varDoc()
	doc.Structure.Routing = game.RouteStrategyOrdered
	errs, _ = lintCodes(doc)
	assert.Empty(t, errs, "in the right order the gate opens")
}

func TestLintVars_ExclusiveChoices(t *testing.T) {
	doc := varDoc()
	gate(doc).Proof.Blocks = []game.BlockDoc{choice(false, "left", "right")}
	door(doc).Depends = game.DependsField{"left", "right"}
	errs, _ := lintCodes(doc)
	assert.Contains(t, errs, "DEPENDS_EXCLUSIVE_CHOICES")

	gate(doc).Proof.Blocks = []game.BlockDoc{choice(true, "left", "right")}
	errs, _ = lintCodes(doc)
	assert.NotContains(t, errs, "DEPENDS_EXCLUSIVE_CHOICES", "multi-select can pick both")
}

func TestLintVars_ProofBlockWaitsTooLate(t *testing.T) {
	doc := varDoc()
	gate(doc).Reveal.Blocks = []game.BlockDoc{{"type": "quiz", "sets": "late"}}
	gate(doc).Proof.Blocks = append(gate(doc).Proof.Blocks, game.BlockDoc{"type": "text", "depends": []any{"late"}})
	_, warns := lintCodes(doc)
	assert.Contains(t, warns, "BLOCK_DEPENDS_TOO_LATE")

	doc = varDoc()
	gate(doc).Proof.Blocks = append(gate(doc).Proof.Blocks, game.BlockDoc{"type": "text", "depends": []any{"key"}})
	errs, warns := lintCodes(doc)
	assert.Empty(t, errs)
	assert.NotContains(t, warns, "BLOCK_DEPENDS_TOO_LATE", "a proof block can wait on a proof block beside it")
}

func TestLintVars_RandomisedWindow(t *testing.T) {
	doc := varDoc()
	doc.Structure.Routing = game.RouteStrategyRandomised
	doc.Structure.MaxNext = 1
	_, warns := lintCodes(doc)
	assert.Contains(t, warns, "DEPENDS_IN_RANDOMISED_WINDOW")
}

func TestLintVars_BandNeedsExclusiveChoices(t *testing.T) {
	branches := func(multi bool) *game.GameDoc {
		doc := varDoc()
		gate(doc).Proof.Blocks = []game.BlockDoc{choice(multi, "left", "right")}
		door(doc).Depends = nil
		door(doc).Routing = game.RouteStrategyFreeRoam
		door(doc).Proof.Blocks = nil
		door(doc).Children = []game.ObjectiveDoc{
			{Slug: "left-path", Title: "Left", Depends: game.DependsField{"left"},
				Proof: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz"}}}},
			{Slug: "right-path", Title: "Right", Depends: game.DependsField{"right"},
				Proof: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz"}}}},
		}
		return doc
	}

	_, warns := lintCodes(branches(false))
	assert.Contains(t, warns, "BAND_NEEDS_EXCLUSIVE_CHOICES")

	_, warns = lintCodes(branches(true))
	assert.NotContains(t, warns, "BAND_NEEDS_EXCLUSIVE_CHOICES")

	doc := branches(false)
	door(doc).ChildrenMin = intPtr(1)
	door(doc).ChildrenMax = intPtr(1)
	_, warns = lintCodes(doc)
	assert.NotContains(t, warns, "BAND_NEEDS_EXCLUSIVE_CHOICES", "one of the two is enough")
}

// The finish page comes last, so a block there can wait on an answer beside it.
func TestLintVars_FinishPageWaitsOnItsOwnAnswer(t *testing.T) {
	doc := varDoc()
	registry := newTestRegistry()
	registry.contexts["quiz"] = append(registry.contexts["quiz"], game.ContextFinish)
	doc.Finish = []game.BlockDoc{
		{"type": "quiz", "sets": "rated_it"},
		{"type": "text", "depends": []any{"rated_it"}},
	}
	result := game.Lint(doc, registry)
	assert.False(t, result.HasError("DEPENDS_UNSATISFIABLE"))
	assert.Empty(t, result.Errors)
}

// A section that finishes at one child closes the moment the key's holder is
// done, so the sibling waiting on that key never opens.
func TestLintVars_BandClosesBeforeTheGateOpens(t *testing.T) {
	doc := varDoc()
	doc.Structure.ChildrenMin = intPtr(1)
	doc.Structure.ChildrenMax = intPtr(1)
	_, warns := lintCodes(doc)
	assert.Contains(t, warns, "BAND_CLOSES_FIRST")

	_, warns = lintCodes(varDoc())
	assert.NotContains(t, warns, "BAND_CLOSES_FIRST", "needing both, the section waits for the door")
}
