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

func TestComplete_OnlyAnnouncesAScoreThatExists(t *testing.T) {
	assert.NotContains(t, renderComplete(t, finishedRun(0, true)), "points")
	assert.NotContains(t, renderComplete(t, finishedRun(80, false)), "points")
}

// The editor shows this page in a phone. A click on the journal link there must do
// nothing, or it would take the whole admin page to the player's journal.
func TestComplete_JournalLinkIsInertInAPreview(t *testing.T) {
	run := finishedRun(0, false)
	inPlay := render(t, Complete(run, nil, map[string]blocks.PlayerState{}, map[string]bool{}, false))
	preview := render(t, Complete(run, nil, map[string]blocks.PlayerState{}, map[string]bool{}, true))

	assert.Contains(t, preview, `<div id="player-nav" inert>`)
	assert.NotContains(t, inPlay, "inert", "a player can follow it")
}
