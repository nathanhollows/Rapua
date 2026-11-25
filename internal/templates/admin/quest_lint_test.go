package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The panel says nothing when there is nothing to say, which is what keeps it
// worth reading. A failed run is a third state: neither clean nor dirty.
func TestQuestLint_SilentWhenClean(t *testing.T) {
	var out strings.Builder
	require.NoError(t, QuestLint(services.QuestLint{}).Render(context.Background(), &out))

	rendered := out.String()
	assert.NotContains(t, rendered, "Needs fixing")
	assert.NotContains(t, rendered, "Worth a look")
	assert.NotContains(t, rendered, "could not run")
}

func TestQuestLint_SaysSoWhenItCouldNotRun(t *testing.T) {
	var out strings.Builder
	require.NoError(t, QuestLint(services.QuestLint{Unavailable: true}).Render(context.Background(), &out))

	assert.Contains(t, out.String(), "could not run",
		"an empty list must not pass for an all-clear")
}

// The quest panel carries what belongs to no row. Anything attached to a row
// belongs on that row, or the author reads a complaint they cannot place.
func TestQuestLint_ShowsOnlyWhatBelongsToNoRow(t *testing.T) {
	lint := services.QuestLint{
		Quest: game.LintResult{
			Errors: []game.LintDiag{{Path: "start[0]", Code: "X", Message: "start page problem"}},
		},
		Objectives: map[string]game.LintResult{
			"row": {Errors: []game.LintDiag{{Code: "Y", Message: "a row problem"}}},
		},
	}

	var out strings.Builder
	require.NoError(t, QuestLint(lint).Render(context.Background(), &out))

	rendered := out.String()
	assert.Contains(t, rendered, "start page problem")
	assert.NotContains(t, rendered, "a row problem", "that one is badged on its row")
	// No row count: it made the panel appear and vanish as lint changed,
	// shifting the tree under the cursor.
	assert.NotContains(t, rendered, "1 objective")
}

// The system page editors take their own share and nobody else's: a missing
// start button on the finish page is a complaint about a page you are not on.
func TestSystemPageLint_ShowsOnlyItsOwnPage(t *testing.T) {
	lint := services.QuestLint{
		Quest: game.LintResult{
			Warnings: []game.LintDiag{
				{Path: "start", Code: "NO_START_BUTTON", Message: "no start button"},
				{Path: "finish[0]", Code: "FORCED_PASS_THROUGH", Message: "finish page note"},
			},
		},
	}

	var out strings.Builder
	require.NoError(t, SystemPageLint(lint, "finish").Render(context.Background(), &out))
	rendered := out.String()
	assert.Contains(t, rendered, "finish page note")
	assert.NotContains(t, rendered, "no start button")
}

// The start page offers its own fix for a missing start button, beside the
// blocks, so the panel above them stays quiet about it. The quest page's row
// still carries it, because nothing there offers one.
func TestSystemPageLint_LeavesTheStartButtonToItsHint(t *testing.T) {
	lint := services.QuestLint{
		Quest: game.LintResult{
			Warnings: []game.LintDiag{{Path: "start", Code: "NO_START_BUTTON", Message: "no start button"}},
		},
	}

	var out strings.Builder
	require.NoError(t, SystemPageLint(lint, "start").Render(context.Background(), &out))
	assert.NotContains(t, out.String(), "no start button")
	assert.NotContains(t, out.String(), "Worth a look", "an empty panel is not drawn")
}

// The out-of-band wrappers are how a panel refreshes after a save. Without the
// swap attribute htmx appends them instead, and the author reads a growing
// stack of stale panels.
func TestLintPanels_SwapInPlace(t *testing.T) {
	var objective, systemPage strings.Builder
	require.NoError(t, ObjectiveLintPanelOOB(services.QuestLint{}, "row").
		Render(context.Background(), &objective))
	require.NoError(t, SystemPageLintOOB(services.QuestLint{}, "start").
		Render(context.Background(), &systemPage))

	for name, rendered := range map[string]string{
		"objective":   objective.String(),
		"system page": systemPage.String(),
	} {
		assert.Contains(t, rendered, `hx-swap-oob="true"`, name+" panel swaps in place")
	}
	for _, slot := range []string{"lint-task", "lint-proof", "lint-reveal"} {
		assert.Contains(t, objective.String(), `id="`+slot+`"`)
	}
	assert.Contains(t, systemPage.String(), `id="system-page-lint"`)
}

// A note about a block belongs with that block's section, numbered the way the
// rail counts; one that names no block falls to the task.
func TestLintForZone_PlacesByPath(t *testing.T) {
	result := game.LintResult{
		Warnings: []game.LintDiag{
			{Path: "structure.children[2].proof.blocks[3].max_rating", Message: "in proof"},
			{Path: "structure.children[2].reveal.blocks[0]", Message: "in reveal"},
			{Path: "structure.children[2].slug", Message: "on the task"},
			{Path: "structure.children[2].proof", Message: "the proof as a whole"},
		},
		Errors: []game.LintDiag{{Path: "objective:somewhere", Message: "structural"}},
	}

	proof := lintForZone(result, "proof")
	require.Len(t, proof.Warnings, 2)
	assert.Equal(t, lintLine{Message: "in proof", Block: 4}, proof.Warnings[0])
	assert.Equal(t, lintLine{Message: "the proof as a whole"}, proof.Warnings[1])

	reveal := lintForZone(result, "reveal")
	require.Len(t, reveal.Warnings, 1)
	assert.Equal(t, 1, reveal.Warnings[0].Block)

	task := lintForZone(result, "task")
	require.Len(t, task.Warnings, 1)
	assert.Equal(t, lintLine{Message: "on the task"}, task.Warnings[0])
	assert.Len(t, task.Errors, 1)
	assert.Empty(t, proof.Errors)
}

// A blank title still needs text to click, or the author cannot reach the
// objective that needs one.
func TestObjectiveTitleOrBlank(t *testing.T) {
	assert.Equal(t, "Untitled task", objectiveTitleOrBlank(""))
	assert.Equal(t, "Untitled task", objectiveTitleOrBlank("  "))
	assert.Equal(t, "Find the chest", objectiveTitleOrBlank("Find the chest"))
}

// The start page has no row in the tree, so its row on the quest page is the
// one place a problem with it can be marked where the author is looking.
func TestSystemPageLink_MarksItsOwnPageOnly(t *testing.T) {
	lint := game.LintResult{Warnings: []game.LintDiag{
		{Path: "start", Code: "NO_START_BUTTON", Message: "start page has no start_button block"},
	}}

	var start, finish strings.Builder
	require.NoError(t, objectiveSystemPageLink("Start", "/admin/quest/start", lint.Filter("start")).
		Render(context.Background(), &start))
	require.NoError(t, objectiveSystemPageLink("Complete", "/admin/quest/complete", lint.Filter("finish")).
		Render(context.Background(), &finish))

	assert.Contains(t, start.String(), "start page has no start_button block")
	assert.NotContains(t, finish.String(), "start_button")
}
