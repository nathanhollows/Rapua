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

// renderEditorWith renders the editor for an objective with the blocks given.
func renderEditorWith(t *testing.T, proof, reveal blocks.Blocks, settings models.QuestSettings) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, EditObjective(EditObjectiveData{
		Settings:     settings,
		Objective:    models.Objective{ID: "obj", Slug: "rose", Title: "Scan the label"},
		ProofBlocks:  proof,
		RevealBlocks: reveal,
	}).Render(context.Background(), &out))
	return out.String()
}

func sampleBlocks() (blocks.Blocks, blocks.Blocks) {
	proof := blocks.Blocks{
		blocks.NewScanBlock(blocks.BaseBlock{ID: "scan-1", OwnerID: "obj"}),
		blocks.NewMarkdownBlock(blocks.BaseBlock{ID: "text-1", OwnerID: "obj"}),
	}
	reveal := blocks.Blocks{blocks.NewMarkdownBlock(blocks.BaseBlock{ID: "text-2", OwnerID: "obj"})}
	return proof, reveal
}

// Each of proof and reveal is one card: the block list, a form, and its own
// phone. The task has no phone: its row is drawn beside its fields.
func TestEditObjective_EachSectionCarriesItsOwnPhone(t *testing.T) {
	html := renderEditor(t)

	assert.Equal(t, 2, strings.Count(html, `class="cf-phone"`))
	assert.Contains(t, html, `data-preview="proof"`)
	assert.Contains(t, html, `data-preview="reveal"`)
	assert.Contains(t, html, `?zone=proof`)
	assert.Contains(t, html, `?zone=reveal`)
	assert.NotContains(t, html, `data-preview="card"`)
	assert.NotContains(t, html, "IntersectionObserver", "nothing watches the scroll to swap phones")
}

// The preview is refetched when blocks change, not when the page scrolls.
func TestEditObjective_PhoneRefreshesWhenBlocksChange(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, "blocks-changed from:body")
	assert.Contains(t, html, "htmx:afterRequest from:.cs-edit", "an autosave in a block form refreshes its phone")
}

// Every block has its form in the page, hidden until selected, so an edit
// still saving when another block is picked is not lost with the form.
func TestEditObjective_KeepsEveryFormInThePage(t *testing.T) {
	proof, reveal := sampleBlocks()
	html := renderEditorWith(t, proof, reveal, models.QuestSettings{})

	for _, id := range []string{"scan-1", "text-1", "text-2"} {
		assert.Contains(t, html, `data-block-form="`+id+`"`)
		assert.Contains(t, html, `data-id="`+id+`"`, "and its row in the list")
	}
	assert.Equal(t, 3, strings.Count(html, `data-block-form="`), "one form per block, not a shared one")
}

// Interactive and static blocks read differently in the list, and carry their
// kind in words for anyone who cannot see the colour.
func TestEditObjective_ListSeparatesInteractiveFromStatic(t *testing.T) {
	proof, reveal := sampleBlocks()
	html := renderEditorWith(t, proof, reveal, models.QuestSettings{})

	assert.Equal(t, 1, strings.Count(html, "rl-int"), "the scan")
	assert.Equal(t, 2, strings.Count(html, "rl-static"), "the two text blocks")
	assert.Contains(t, html, `title="Interactive: players do something"`)
	assert.Contains(t, html, `title="Static content"`)
}

// Choosing a block works while a running quest is locked: looking at a block
// edits nothing, and a locked page that cannot be read helps no one.
func TestEditObjective_ListStaysUsableUnderTheLock(t *testing.T) {
	proof, reveal := sampleBlocks()
	html := renderEditorWith(t, proof, reveal, models.QuestSettings{})

	for _, item := range strings.Split(html, `<button type="button" data-ignore-lock data-id="`)[1:] {
		assert.Contains(t, item[:200], "rl-item", "each list row is exempt from the lock")
	}
}

// Reveal is optional: with none, a player goes straight back to the list, and
// the author is told so where the blocks would be.
func TestEditObjective_EmptyRevealExplainsItself(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, "No reveal")
	assert.Contains(t, html, "straight back to the quest list")
	assert.Contains(t, html, "No proof. Players open straight to the reveal.")
}

// Moving between the halves is a drag, so the page carries no button or hint
// for it.
func TestEditObjective_NoMoveButtonOrDragHint(t *testing.T) {
	proof, reveal := sampleBlocks()
	html := renderEditorWith(t, proof, reveal, models.QuestSettings{})

	assert.NotContains(t, html, "data-move")
	assert.NotContains(t, html, "Move to ")
	assert.NotContains(t, html, "Drag to reorder")
}

// The chooser fills its own section's forms, and starts out hidden: it is shown
// where a form would be, by choosing Add.
func TestEditObjective_ChooserTargetsItsSection(t *testing.T) {
	html := renderEditor(t)

	assert.Contains(t, html, `hx-target="#forms-proof"`)
	assert.Contains(t, html, `hx-target="#forms-reveal"`)
	assert.Contains(t, html, `data-add-form="proof"`)
	assert.Contains(t, html, `data-bsearch`, "and can be searched")
}

// The points picker offers these, and the page hides the real field it writes to.
func TestEditObjective_PointsPickerOffersThePresets(t *testing.T) {
	proof, reveal := sampleBlocks()
	html := renderEditorWith(t, proof, reveal, models.QuestSettings{EnablePoints: true})

	for _, v := range []string{"0", "1", "2", "5", "10", "20", "50", "100"} {
		assert.Contains(t, html, `data-set="`+v+`"`)
	}
	assert.Contains(t, html, `points-field`, "so the stylesheet can hide the field the pill writes to")
}

// An empty proof offers templates for the common things a player is asked to
// do, and a way in to the chooser. Reveal is optional and offers none.
func TestEditObjective_EmptyProofOffersSectionTemplates(t *testing.T) {
	html := renderEditor(t)

	for _, kit := range []string{"scan", "location", "quiz"} {
		assert.Contains(t, html, `name="kit" value="`+kit+`"`)
	}
	assert.Equal(t, 3, strings.Count(html, `hx-post="/admin/blocks/kit"`), "one card for each template")
	assert.Contains(t, html, "Build it yourself")
	assert.Contains(t, html, `hx-target="#forms-proof"`)
	// Reveal's own empty state has no templates: its forms never post a kit.
	_, reveal, found := strings.Cut(html, `data-empty-form="reveal"`)
	require.True(t, found)
	revealSection, _, _ := strings.Cut(reveal, "</section>")
	assert.NotContains(t, revealSection, "/admin/blocks/kit")
}

// A block just added arrives with its form, and says whether it is the one to
// select, so a template of several puts the cursor in the first that matters.
func TestObjectiveBlockAdded_SaysWhetherToSelectIt(t *testing.T) {
	scan := blocks.NewScanBlock(blocks.BaseBlock{ID: "scan-1", OwnerID: "obj"})
	text := blocks.NewMarkdownBlock(blocks.BaseBlock{ID: "text-1", OwnerID: "obj"})

	render := func(b blocks.Block, selected bool) string {
		var out strings.Builder
		require.NoError(t, ObjectiveBlockAdded(models.QuestSettings{}, b, blocks.ContextObjectiveProof, selected).
			Render(context.Background(), &out))
		return out.String()
	}
	assert.Contains(t, render(scan, true), `data-rail-for="scan-1"`)
	assert.Contains(t, render(scan, true), "data-select")
	assert.NotContains(t, render(text, false), "data-select")
}
