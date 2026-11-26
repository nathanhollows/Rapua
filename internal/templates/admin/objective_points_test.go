package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/models"
)

func renderBlockForm(t *testing.T, block blocks.Block) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, ObjectiveBlockAdded(models.QuestSettings{EnablePoints: true}, block, blocks.ContextObjectiveProof, false).
		Render(context.Background(), &out))
	return out.String()
}

// The header's picker owns a block's points and sends them with the block's own
// form, so the input has to point at that form by its id.
func TestBlockHeader_OwnsThePointsInput(t *testing.T) {
	quiz := renderBlockForm(t, &blocks.QuizBlock{BaseBlock: blocks.BaseBlock{ID: "q1", Points: 3}})
	assert.Contains(t, quiz, `form="form-q1"`)
	assert.Contains(t, quiz, `data-pt-input`)
	assert.Contains(t, quiz, `id="form-q1"`, "the form the points input names has to exist")
	assert.Equal(t, 1, strings.Count(quiz, `name="points"`), "one points input, not one per template")
	assert.Contains(t, quiz, `data-set="5"`)
	assert.NotContains(t, quiz, `data-cost`)

	// Nothing to award for a block that asks nothing of the player.
	text := renderBlockForm(t, &blocks.MarkdownBlock{BaseBlock: blocks.BaseBlock{ID: "t1"}})
	assert.NotContains(t, text, `name="points"`)
}

// A clue is paid for, so its picker offers the usual values as costs.
func TestBlockHeader_ClueOffersCosts(t *testing.T) {
	clue := renderBlockForm(t, &blocks.ClueBlock{BaseBlock: blocks.BaseBlock{ID: "c1", Points: -15}})
	assert.Contains(t, clue, `data-cost`)
	assert.Contains(t, clue, `value="-15"`)
	for _, cost := range []string{"-1", "-2", "-5", "-10", "-20", "-50", "-100"} {
		assert.Contains(t, clue, `data-set="`+cost+`"`)
	}
	assert.NotContains(t, clue, `data-set="5"`, "positive values would pay the team to take a hint")
	assert.Contains(t, clue, "Taken from the team when they reveal the clue")
}
