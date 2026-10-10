package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// One preview, not one per zone. Three sticky phones meant most of them were
// off screen most of the time and the page was three screens tall before any
// blocks were expanded.
func TestEditObjective_HasASinglePreview(t *testing.T) {
	html := renderEditor(t)

	assert.Equal(t, 1, strings.Count(html, "mockup-phone bg-black"))
}

// The one phone holds all three views and shows the one the author is scrolled
// to, so switching costs no request and the proof preview is not refetched
// every time it comes back into view.
func TestEditObjective_PreviewHoldsEveryView(t *testing.T) {
	html := renderEditor(t)

	for _, zone := range []string{"card", "proof", "reveal"} {
		assert.Contains(t, html, `data-preview="`+zone+`"`)
		assert.Contains(t, html, `data-zone="`+zone+`"`, "and the form section it belongs to")
	}
}

// The pills say which view is loaded, because a preview that changes on its own
// without saying so reads as a glitch. They are also the way back: clicking one
// scrolls the form to that zone.
func TestEditObjective_PillsIndicateAndNavigate(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, `id="preview-pills"`)
	assert.Equal(t, 3, strings.Count(html, `data-pill="`))
	assert.Contains(t, html, `data-pill="card"`)
}

// Reordering inside a preview still works: a pane is one zone, so the list its
// order belongs to is never in doubt. Only dragging a block from one zone to
// another is out, and that was never on offer here.
func TestEditObjective_PreviewPaneMirrorsItsOwnList(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, `data-blocks-target="content-blocks"`)
	assert.Contains(t, html, `data-blocks-target="nav-blocks"`)
	assert.Equal(t, 2, strings.Count(html, `data-blocks-target="`),
		"the card pane has no list of blocks to reorder")
}
