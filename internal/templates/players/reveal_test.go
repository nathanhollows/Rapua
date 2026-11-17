package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

// The reveal offers no next-task button: the next task may be in another group,
// and the list is where that choice is made.
func TestObjectiveView_RevealOffersOnlyTheWayBack(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{
		Title: "Ask at the fale who the space is named for",
		Zone:  game.ContextObjectiveReveal,
	}))

	assert.Contains(t, html, `href="/objectives"`)
	assert.Contains(t, html, "Back to the quest")
	assert.NotContains(t, html, "/objective/", "no link to another task")
}

// A short page still ends with the way back at the bottom, via a flex column.
func TestObjectiveView_TheWayBackIsPinnedToTheBottom(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{Title: "T", Zone: game.ContextObjectiveReveal}))

	assert.Contains(t, html, "q-page")
	assert.Contains(t, html, "mt-auto")
}

// Proof has the way back too: a player stuck on a task needs a way out that is
// not the dock.
func TestObjectiveView_ProofHasTheWayBackToo(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{Title: "Scan the tag", Zone: game.ContextObjectiveProof}))

	assert.Contains(t, html, "Back to the quest")
}

// Neutral on the reveal, tinted on the proof. The description shows only on the
// proof: it is read on arrival.
func TestObjectiveView_RevealIsNeutralAndProofTakesTheSectionsHue(t *testing.T) {
	data := ObjectiveViewData{Title: "T", Description: "Second bed on the left", Hue: HueFor(2)}

	data.Zone = game.ContextObjectiveProof
	proof := render(t, ObjectiveView(data))
	assert.Contains(t, proof, hueClass(HueFor(2)))
	assert.NotContains(t, proof, "q-neutral")
	assert.Contains(t, proof, "Second bed on the left")

	data.Zone = game.ContextObjectiveReveal
	reveal := render(t, ObjectiveView(data))
	assert.Contains(t, reveal, "q-neutral")
	assert.NotContains(t, reveal, hueClass(HueFor(2)))
	assert.NotContains(t, reveal, "Second bed on the left")
}

// A preview is a picture of the page: no footer, no toast slot.
func TestObjectiveView_PreviewHasNoFooterOrToast(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{Title: "T", Zone: game.ContextObjectiveReveal, Preview: true}))

	assert.NotContains(t, html, "Back to the quest")
	assert.NotContains(t, html, "reveal-toast")
}

// The hand-off waits a beat, can be hurried by a tap, and refetches the page to
// draw the reveal.
func TestRevealToast_HandsOverAfterABeatAndCanBeTapped(t *testing.T) {
	html := render(t, RevealToast("garden"))

	assert.Contains(t, html, `id="reveal-toast"`)
	assert.Contains(t, html, `hx-swap-oob="true"`)
	assert.Contains(t, html, `hx-get="/objective/garden"`)
	assert.Contains(t, html, `hx-trigger="load delay:1700ms, click"`)
	assert.Contains(t, html, `hx-select="#objective-page"`)
	assert.Contains(t, html, `aria-live="polite"`)
}

func TestHueOfObjective_IsTheColourTheListGaveItsSection(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("a", "root", "A"), obj("a1", "a", "A1"),
		obj("b", "root", "B"), obj("b1", "b", "B1"),
		obj("lone", "root", "Lone"),
	)

	assert.Equal(t, HueFor(1), HueOfObjective(rows, rows[4]), "b1 sits in the second section")
	assert.Equal(t, HueFor(0), HueOfObjective(rows, rows[2]))
	assert.Zero(t, HueOfObjective(rows, rows[5]), "a task under no section keeps the default")
}

// An unknown lucide name renders as nothing, so a typo is a blank tile and not
// an error. Every icon the player pages ask for must exist.
func TestPlayerIcons_AllExist(t *testing.T) {
	for _, name := range []string{
		"check", "circle-x", "camera", "bell", "flag", "footprints", "trophy", "compass",
		"pencil", "book-open", "circle-dot", "chevron-right", "hourglass", "circle-check-big",
	} {
		assert.Contains(t, render(t, icon(name, nil)), "<svg", "icon %q", name)
	}
}
