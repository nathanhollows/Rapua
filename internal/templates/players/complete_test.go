package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func finishedRun(points int, enabled bool) models.Run {
	return models.Run{
		Code:   "EPFH",
		Name:   "Nathan Nathan",
		Points: points,
		Quest:  models.Quest{Name: "The Perfumer's Garden", Settings: models.QuestSettings{EnablePoints: enabled}},
	}
}

func renderComplete(t *testing.T, run models.Run) string {
	t.Helper()
	return render(t, Complete(run, nil, map[string]blocks.PlayerState{}, map[string]bool{}, false))
}

// The last page of a game carries no footer line.
func TestComplete_HasNoFooterLine(t *testing.T) {
	html := renderComplete(t, finishedRun(80, true))

	assert.NotContains(t, html, "Rules")
	assert.NotContains(t, html, "EPFH", "the code is on the You tab")
	assert.NotContains(t, html, "Nathan Nathan")
	assert.NotContains(t, html, "<footer")
}

func TestComplete_SaysWhatWasFinishedAndTheScore(t *testing.T) {
	html := renderComplete(t, finishedRun(80, true))

	assert.Contains(t, html, "You finished The Perfumer&#39;s Garden.")
	assert.Contains(t, html, "80 points")
	assert.Contains(t, html, `href="/journal"`)
}

// A score of nothing is not announced, and neither is one nobody is keeping.
func TestComplete_OnlyAnnouncesAScoreThatExists(t *testing.T) {
	assert.NotContains(t, renderComplete(t, finishedRun(0, true)), "points")
	assert.NotContains(t, renderComplete(t, finishedRun(80, false)), "points")
}
