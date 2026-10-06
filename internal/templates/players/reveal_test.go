package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

// A reveal offers the next task rather than returning the player to the list.
func TestObjectiveView_RevealOffersTheWayOn(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{
		Title: "Ask at the fale who the space is named for",
		Zone:  game.ContextObjectiveReveal,
		Next:  NextUp{Title: "Find which floor holds the maps", Slug: "maps"},
	}))

	assert.Contains(t, html, "Find which floor holds the maps")
	assert.Contains(t, html, `href="/objective/maps"`)
	assert.Contains(t, html, `href="/objectives"`, "and a way back to the whole list")
}

// Without a next task the last reveal still needs a way out.
func TestObjectiveView_RevealWithNothingNextStillLeads(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{
		Title: "The last one", Zone: game.ContextObjectiveReveal,
	}))

	assert.Contains(t, html, `href="/objectives"`)
	assert.NotContains(t, html, "/objective/")
}

// Proof offers no way on: the way forward is to do the task.
func TestObjectiveView_ProofOffersNoWayOn(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{
		Title: "Scan the tag", Zone: game.ContextObjectiveProof,
		Next: NextUp{Title: "Something else", Slug: "else"},
	}))

	assert.NotContains(t, html, "Something else")
	assert.NotContains(t, html, `href="/objectives"`)
}
