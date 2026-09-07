package services_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
)

// Partitioning is the whole point of QuestLint: a diagnostic that lands in the
// wrong bucket appears on a page that cannot act on it, or on none at all.
// These check the buckets directly, since the fixture's lint helper flattens
// everything back into one list and cannot tell them apart.

func questLintFixture() services.QuestLint {
	return services.QuestLint{
		RootID: "root-id",
		Quest: game.LintResult{
			Errors: []game.LintDiag{
				{Path: "start[0]", Code: "INVALID_CONTEXT", Message: "start block wrong"},
				{Path: "quest", Code: services.LintSeveralRoots, Message: "two roots"},
				{Path: "objective:stranded", Code: services.LintOrphaned, Message: "stranded"},
			},
			Warnings: []game.LintDiag{
				{Path: "finish[1]", Code: "NESTING_TOO_DEEP", Message: "finish page note"},
			},
		},
		Objectives: map[string]game.LintResult{
			"row-id": {Errors: []game.LintDiag{{Code: "BAND_OUT_OF_RANGE", Message: "band"}}},
		},
	}
}

// Filter is how the start and finish editors take their own share of the
// quest-level diagnostics, and nobody else's.
func TestQuestLint_FilterSplitsTheSystemPages(t *testing.T) {
	lint := questLintFixture()

	start := lint.Quest.Filter("start")
	assert.Len(t, start.Errors, 1)
	assert.Equal(t, "start block wrong", start.Errors[0].Message)
	assert.Empty(t, start.Warnings, "the finish page's warning is not the start page's")

	finish := lint.Quest.Filter("finish")
	assert.Empty(t, finish.Errors)
	assert.Len(t, finish.Warnings, 1)
}

// An ordinary objective's page shows its own diagnostics and nothing else.
func TestQuestLint_ForEditorGivesARowItsOwn(t *testing.T) {
	result := questLintFixture().ForEditor("row-id")

	assert.True(t, result.HasError("BAND_OUT_OF_RANGE"))
	assert.False(t, result.HasError("INVALID_CONTEXT"), "the start page is not this row's problem")
	assert.False(t, result.HasError(services.LintSeveralRoots))
}

// The root's page is the only surface carrying the controls the quest's own
// diagnostics are about, but not the start and finish pages: those have
// editors of their own, and a missing start button helps nobody here.
func TestQuestLint_ForEditorGivesTheRootTheQuestsShape(t *testing.T) {
	result := questLintFixture().ForEditor("root-id")

	assert.True(t, result.HasError(services.LintSeveralRoots), "the shape of the tree")
	assert.True(t, result.HasError(services.LintOrphaned), "and the rows nothing reaches")
	assert.False(t, result.HasError("INVALID_CONTEXT"), "but not the start page's blocks")
	assert.False(t, result.HasWarning("NESTING_TOO_DEEP"), "nor the finish page's")
}

// A run that never happened is not a clean one, and every accessor has to say
// so the same way.
func TestQuestLint_UnavailableStaysUnavailableThroughTheAccessors(t *testing.T) {
	lint := services.QuestLint{Unavailable: true}

	assert.False(t, lint.IsValid())
	assert.True(t, lint.Unreadable())
	assert.Empty(t, lint.ForEditor("anything").Errors, "there is nothing to report, not nothing wrong")
}
