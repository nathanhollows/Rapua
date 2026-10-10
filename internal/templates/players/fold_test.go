package templates

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// A place is not foldable: it is where the run is standing, so there is nothing
// to collapse it into. The nesting it replaced is carried by the path instead,
// which costs no width and so needs no escape hatch.
func TestQuestPlace_DoesNotFold(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("wing", "root", "Meet the people who keep Te Aro running"),
		obj("a", "wing", "Ask at the fale"), obj("b", "wing", "Find the maps"),
	)

	html := render(t, Objectives(ObjectivesParams{
		View: viewOf([]models.Objective{rows[2], rows[3]}, rows),
	}))

	assert.NotContains(t, html, "<details")
	assert.NotContains(t, html, "objective-fold")
	assert.Contains(t, html, "Meet the people who keep Te Aro running")
	assert.Equal(t, 1, strings.Count(html, "Ask at the fale"))
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
