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
	assert.Contains(t, rendered, "Needs fixing: 1 objective", "and counted here")
}

// The system page editors take their own share and nobody else's: a missing
// start button on the finish page is a complaint about a page you are not on.
func TestSystemPageLint_ShowsOnlyItsOwnPage(t *testing.T) {
	lint := services.QuestLint{
		Quest: game.LintResult{
			Warnings: []game.LintDiag{
				{Path: "start", Code: "NO_START_BUTTON", Message: "no start button"},
				{Path: "finish[0]", Code: "NESTING_TOO_DEEP", Message: "finish page note"},
			},
		},
	}

	var out strings.Builder
	require.NoError(t, SystemPageLint(lint, "start").Render(context.Background(), &out))
	rendered := out.String()
	assert.Contains(t, rendered, "no start button")
	assert.NotContains(t, rendered, "finish page note")
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
	assert.Contains(t, objective.String(), `id="objective-lint"`)
	assert.Contains(t, systemPage.String(), `id="system-page-lint"`)
}
