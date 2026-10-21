package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

func gatedText(id, content string, deps ...string) *blocks.MarkdownBlock {
	b := &blocks.MarkdownBlock{BaseBlock: blocks.BaseBlock{ID: id, Depends: deps}, Content: content}
	return b
}

// A gated block keeps its place but gives nothing away: what a gate holds back
// is usually the answer to something.
func TestObjectiveView_HiddenBlockLeavesAnEmptySlot(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{
		Title:  "The lock",
		Zone:   game.ContextObjectiveProof,
		Blocks: blocks.Blocks{gatedText("secret", "The code is 4417", "key")},
		Hidden: map[string]bool{"secret": true},
	}))

	assert.Contains(t, html, `id="preview-block-secret"`)
	assert.NotContains(t, html, "4417")
}

func TestObjectiveView_PreviewShowsWhatAGateWaitsFor(t *testing.T) {
	html := render(t, ObjectiveView(ObjectiveViewData{
		Title:   "The lock",
		Zone:    game.ContextObjectiveProof,
		Blocks:  blocks.Blocks{gatedText("secret", "The code is 4417", "key", "lamp")},
		Preview: true,
	}))

	assert.Contains(t, html, "4417")
	assert.Contains(t, html, "Shown after key, lamp")
}
