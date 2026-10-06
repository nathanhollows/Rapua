package templates

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// A section folds rather than being trimmed, so every row the author wrote stays.
func TestQuestSection_Folds(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("wing", "root", "Meet the people who keep Te Aro running"),
		obj("a", "wing", "Ask at the fale"), obj("b", "wing", "Find the maps"),
	)

	html := render(t, Objectives(ObjectivesParams{
		View: viewOf([]models.Objective{rows[2], rows[3]}, rows),
	}))

	assert.Contains(t, html, "<details", "the fold is markup, so it works before any script runs")
	assert.Contains(t, html, `data-fold="wing"`, "keyed by the section, so the choice is remembered")
	assert.Contains(t, html, "<summary", "the header is the control")
	assert.Equal(t, 2, strings.Count(html, "Ask at the fale")+strings.Count(html, "Find the maps"))
}

// Open by default so a player sees what is in the quest.
func TestQuestSection_StartsOpen(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("wing", "root", "A section"), obj("a", "wing", "A row"),
	)

	html := render(t, Objectives(ObjectivesParams{
		View: viewOf([]models.Objective{rows[2]}, rows),
	}))
	assert.Contains(t, html, "<details open")
}

// A standalone row is not a section and has nothing to fold.
func TestQuestRow_StandaloneDoesNotFold(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("solo", "root", "One task"))

	html := render(t, Objectives(ObjectivesParams{
		View: viewOf([]models.Objective{rows[1]}, rows),
	}))
	assert.NotContains(t, html, "<details")
}

// The scanner shows only when something on offer can be scanned.
func TestObjectives_OffersTheScannerOnlyWhenSomethingWantsIt(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("a", "root", "Scan the tag"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	assert.NotContains(t, render(t, Objectives(ObjectivesParams{View: view})), "/scan")

	view.OfferedProof = map[string]bool{"scan": true}
	html := render(t, Objectives(ObjectivesParams{View: view}))
	assert.Contains(t, html, `href="/scan"`)
	assert.Contains(t, html, "Scan a code")
	// In the flow, not pinned: a fixed bar would hide the last row.
	assert.NotContains(t, html, "fixed inset-x-0")
	assert.Greater(t, strings.Index(html, "/scan"), strings.Index(html, "Scan the tag"))
}
