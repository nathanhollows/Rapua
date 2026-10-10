package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderEditor(t *testing.T) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, EditObjective(EditObjectiveData{
		Objective: models.Objective{ID: "obj", Slug: "rose", Title: "Scan the label"},
	}).Render(context.Background(), &out))
	return out.String()
}

// The preview is one phone showing the zone the author is scrolled to. The
// tabs that used to point one preview at two zones are gone, and so is the
// pair of previews that briefly replaced them.
func TestEditObjective_PreviewFollowsTheScroll(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, `?zone=proof`)
	assert.Contains(t, html, `?zone=reveal`)
	assert.Contains(t, html, "IntersectionObserver", "the browser decides what is on screen")
	assert.NotContains(t, html, "changePreviewPage")
}

// The admin lists keep the ids the preview's drag mirror writes back into. A
// rename here silently stops reordering from reaching the form.
func TestEditObjective_KeepsTheIdsTheDragMirrorNeeds(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, `id="content-blocks"`)
	assert.Contains(t, html, `id="nav-blocks"`)
}

// The add control sits in the zone it adds to and names it, rather than
// floating between two zones belonging to neither.
func TestEditObjective_AddControlNamesItsZone(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, "Add to Proof")
	assert.Contains(t, html, "Add to Reveal")
	assert.NotContains(t, html, ">Add content")
}

// The title is edited far more often than anything else on this page and has
// no preview at all: an author cannot see the row they are writing. The card
// at the top is that row.
func TestEditObjective_ShowsTheRowThePlayerWillSee(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, `id="objective-card"`)
	assert.Contains(t, html, `id="objective-card-title"`)
	assert.Contains(t, html, "Scan the label", "the card carries the objective's own words")
}

// The card's icon is the first interactive block in the proof, the same rule
// the player's own row uses. An objective that asks for nothing shows nothing.
func TestEditObjective_CardBorrowsTheProofBlockIcon(t *testing.T) {
	var out strings.Builder
	require.NoError(t, EditObjective(EditObjectiveData{
		Objective: models.Objective{ID: "obj", Slug: "rose", Title: "Scan it"},
		ProofBlocks: blocks.Blocks{
			blocks.NewMarkdownBlock(blocks.BaseBlock{}),
			blocks.NewScanBlock(blocks.BaseBlock{}),
		},
	}).Render(context.Background(), &out))

	assert.Contains(t, out.String(), "scan-qr-code", "text asks nothing, so the scan is the icon")
}

// One title on screen, not two. The heading was a copy of the field below it,
// kept in step by script: two of the same string, and a line of hyperscript to
// stop them disagreeing.
func TestEditObjective_HasNoDuplicateTitle(t *testing.T) {
	html := renderEditor(t)

	assert.NotContains(t, html, `id="objective-title"`)
	// The heading names the kind of page, the way every other page here does.
	// It used to be a copy of the title, kept in step by script.
	assert.Contains(t, html, `class="text-2xl font-bold">Objective</h1>`)
	// Twice is right: the field holds the title and the card shows it. The card
	// has to be correct on load, not only once somebody types.
	assert.Equal(t, 2, strings.Count(html, "Scan the label"))
}

// Saving is automatic, so a button that only exists to be clicked by a trigger
// is a control that promises a decision the author never makes.
func TestEditObjective_SavesWithoutASaveButton(t *testing.T) {
	html := renderEditor(t)

	assert.NotContains(t, html, "edit-objective-btn")
	assert.Contains(t, html, "keyup", "typing still saves")
	assert.Contains(t, html, "change from:[form=edit-objective]")
	assert.Contains(t, html, `id="save-state"`, "and says so, since nothing else would")
}

// The zone heading belongs to the blocks, not to the whole band: spanning the
// preview column it pinned on top of the phone and cut its head off.
func TestEditObjective_ZoneHeadingDoesNotCoverThePreview(t *testing.T) {
	html := renderEditor(t)

	assert.NotContains(t, html, "lg:col-span-2")
}

// The description is the detail a player needs on arrival, and it saves like
// every other field here: named, in the form, echoing into the card.
func TestEditObjective_DescriptionIsARealField(t *testing.T) {
	var out strings.Builder
	require.NoError(t, EditObjective(EditObjectiveData{
		Objective: models.Objective{
			ID: "obj", Slug: "rose", Title: "Scan the label",
			Description: "Second bed on the left.",
		},
	}).Render(context.Background(), &out))

	html := out.String()
	at := strings.Index(html, `id="description"`)
	require.GreaterOrEqual(t, at, 0)
	tag := html[strings.LastIndex(html[:at], "<"):]
	tag = tag[:strings.Index(tag, ">")]
	assert.Contains(t, tag, `name="description"`, "it posts")
	assert.Contains(t, tag, `form="edit-objective"`, "with the rest of the objective")
	assert.Contains(t, html, "objective-card-description's textContent", "and echoes into the card")
	// Twice: the field holds it and the card shows it, both correct on load.
	assert.Equal(t, 2, strings.Count(html, "Second bed on the left."))
	// The slug row still says "Not saved yet" and should: that one is still
	// simulated. The description no longer carries the warning or the badge.
	assert.NotContains(t, html, "badge-warning")
}

// An objective with no description shows no second line rather than an empty
// one, so the card is the row the player will actually see.
func TestEditObjective_CardHidesAnEmptyDescription(t *testing.T) {
	html := renderEditor(t)

	at := strings.Index(html, `id="objective-card-description"`)
	require.GreaterOrEqual(t, at, 0)
	assert.Contains(t, html[at:at+160], "hidden")
}

// The link is shown because a player can read it, and a title that names the
// task gives the answer away in the address bar. It is shown and not editable:
// a control that does nothing is worse than no control.
func TestEditObjective_ShowsTheObjectiveLink(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, "/objective/rose")
	assert.Contains(t, html, "Players can see this")
	assert.NotContains(t, html, `id="slug-input"`)
	assert.NotContains(t, html, "not wired")
}


// A row on its own says what it is; a row inside its container says what it is
// part of. The preview draws the container when there is one.
func TestEditObjective_PreviewShowsTheContainer(t *testing.T) {
	var out strings.Builder
	require.NoError(t, EditObjective(EditObjectiveData{
		Objective:   models.Objective{ID: "obj", Slug: "rose", Title: "Rose"},
		ParentTitle: "Choose your heart note",
		ParentRule:  "Any one of three",
	}).Render(context.Background(), &out))

	html := out.String()
	assert.Contains(t, html, "Choose your heart note")
	assert.Contains(t, html, "Any one of three")
}

// The root is the quest rather than a place in it, so an objective directly
// under it gets no card: drawing one would invent a container.
func TestEditObjective_PreviewHasNoContainerUnderTheRoot(t *testing.T) {
	html := renderEditor(t)

	assert.NotContains(t, html, "objective-fold", "no section shell without a section")
}
