package blocks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanhollows/Rapua/v8/blocks"
	bTemplates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Points live in the editor's header picker and nowhere in a block's own form.
// A form that carried its own copy could disagree with the picker, and did:
// thirteen templates each kept one.
func TestAdminForms_CarryNoPointsField(t *testing.T) {
	for _, registered := range blocks.GetRegisteredBlocks() {
		block := registered.Prototype
		var out strings.Builder
		err := bTemplates.RenderAdminEdit(models.QuestSettings{EnablePoints: true}, block).
			Render(context.Background(), &out)
		require.NoError(t, err, block.GetType())
		assert.NotContains(t, out.String(), `name="points"`,
			"%s form has its own points field", block.GetType())
	}
}

// A limit the author set outside the offered choices (an imported game can hold
// any number) stays selected instead of being rewritten by the next save.
func TestGeofenceAdmin_AccuracyChoices(t *testing.T) {
	render := func(accuracy float64) string {
		var out strings.Builder
		block := blocks.GeofenceBlock{BaseBlock: blocks.BaseBlock{ID: "g"}, MaxAccuracy: accuracy}
		require.NoError(t, bTemplates.RenderAdminEdit(models.QuestSettings{}, &block).
			Render(context.Background(), &out))
		return out.String()
	}

	unset := render(0)
	assert.Contains(t, unset, `<option value="50" selected>`, "no limit set shows the default")
	assert.Contains(t, unset, "outdoors, open sky")
	assert.NotContains(t, unset, "set elsewhere")

	outdoors := render(10)
	assert.Contains(t, outdoors, `<option value="10" selected>`)

	odd := render(37)
	assert.Contains(t, odd, `<option value="37" selected>37 m: set elsewhere</option>`)
}

// Reordering is by dragging, so no list form carries move buttons.
func TestAdminForms_ListsAreDraggableRows(t *testing.T) {
	forms := map[string]blocks.Block{
		"choice":      &blocks.ChoiceBlock{BaseBlock: blocks.BaseBlock{ID: "b"}},
		"quiz":        &blocks.QuizBlock{BaseBlock: blocks.BaseBlock{ID: "b"}},
		"sorting":     &blocks.SortingBlock{BaseBlock: blocks.BaseBlock{ID: "b"}},
		"checklist":   &blocks.ChecklistBlock{BaseBlock: blocks.BaseBlock{ID: "b"}},
		"random_text": &blocks.RandomTextBlock{BaseBlock: blocks.BaseBlock{ID: "b"}},
		"scan":        &blocks.ScanBlock{BaseBlock: blocks.BaseBlock{ID: "b"}},
	}
	for name, block := range forms {
		var out strings.Builder
		require.NoError(t, bTemplates.RenderAdminEdit(models.QuestSettings{}, block).
			Render(context.Background(), &out), name)
		html := out.String()
		assert.Contains(t, html, "data-rows", name)
		assert.Contains(t, html, "rows-grip", name)
		assert.Contains(t, html, `data-rows-template="`, name)
		assert.NotContains(t, html, "Move up", name)
		assert.NotContains(t, html, "Move down", name)
	}
}

// The quiz matches marks to answers by value, so each mark must be renumbered with its row.
func TestQuizAdmin_MarksFollowTheirRows(t *testing.T) {
	var out strings.Builder
	block := &blocks.QuizBlock{
		BaseBlock: blocks.BaseBlock{ID: "q"},
		Options: []blocks.QuizOption{
			{ID: "option_0", Text: "A", IsCorrect: true},
			{ID: "option_1", Text: "B"},
		},
	}
	require.NoError(t, bTemplates.RenderAdminEdit(models.QuestSettings{}, block).
		Render(context.Background(), &out))
	assert.Equal(t, 3, strings.Count(out.String(), `data-rows-index="option_"`), "two rows and the blank one")
	assert.Contains(t, out.String(), `value="option_1"`)
}

// Prose fields open tall enough to write in; a one-line box invites a one-line answer.
func TestAdminForms_ProseFieldsOpenTall(t *testing.T) {
	render := func(block blocks.Block) string {
		var out strings.Builder
		require.NoError(t, bTemplates.RenderAdminEdit(models.QuestSettings{}, block).
			Render(context.Background(), &out))
		return out.String()
	}
	assert.Contains(t, render(&blocks.MarkdownBlock{BaseBlock: blocks.BaseBlock{ID: "t"}}), "min-height: calc(10lh")
	assert.Contains(t, render(&blocks.ToggleTextBlock{BaseBlock: blocks.BaseBlock{ID: "t"}}), "min-height: calc(10lh")
	assert.Contains(t, render(&blocks.AlertBlock{BaseBlock: blocks.BaseBlock{ID: "a"}}), "min-height: calc(2lh")
}

// Without typing there is no box to type in: a hidden one would still be on the page.
func TestScanPlayer_TypingBoxFollowsTheSwitch(t *testing.T) {
	render := func(allow bool) string {
		block := blocks.ScanBlock{BaseBlock: blocks.BaseBlock{ID: "s"}, AllowTyping: allow}
		return player(t, bTemplates.RenderPlayerView(models.QuestSettings{}, &block, open()))
	}
	assert.NotContains(t, render(false), `name="code"`)
	assert.Contains(t, render(true), `name="code"`)
}
