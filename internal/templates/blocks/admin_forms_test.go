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
