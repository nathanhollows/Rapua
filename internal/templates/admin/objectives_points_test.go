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

func TestEditObjectiveData_TotalPoints(t *testing.T) {
	t.Run("sums points across proof and reveal blocks", func(t *testing.T) {
		data := EditObjectiveData{
			ProofBlocks: blocks.Blocks{
				blocks.NewMarkdownBlock(blocks.BaseBlock{Points: 10}),
			},
			RevealBlocks: blocks.Blocks{
				blocks.NewMarkdownBlock(blocks.BaseBlock{Points: 20}),
			},
		}
		assert.Equal(t, 30, data.TotalPoints())
	})

	t.Run("no blocks is zero", func(t *testing.T) {
		assert.Equal(t, 0, EditObjectiveData{}.TotalPoints())
	})
}

// An objective with no proof blocks has nothing to show on Proof, so opening
// there would put an empty phone screen in front of the author.
func TestEditObjectiveData_DefaultZoneFollowsTheContent(t *testing.T) {
	proof := EditObjectiveData{
		ProofBlocks: blocks.Blocks{blocks.NewMarkdownBlock(blocks.BaseBlock{})},
	}
	assert.Equal(t, "proof", proof.DefaultZone())

	revealOnly := EditObjectiveData{
		RevealBlocks: blocks.Blocks{blocks.NewMarkdownBlock(blocks.BaseBlock{})},
	}
	assert.Equal(t, "reveal", revealOnly.DefaultZone(),
		"nothing to prove, so the reveal is the whole of what this objective shows")

	assert.Equal(t, "reveal", EditObjectiveData{}.DefaultZone(),
		"an empty objective opens where its first block is most likely to go")
}

// classOf returns only the class attribute, not the whole tag: the tabs'
// hyperscript also names .tab-active, which would give false matches.
func classOf(t *testing.T, html, id string) string {
	t.Helper()
	at := strings.Index(html, `id="`+id+`"`)
	require.GreaterOrEqual(t, at, 0, "no element with id %q", id)
	from := strings.Index(html[at:], ` class="`)
	require.GreaterOrEqual(t, from, 0, "element %q has no class attribute", id)
	start := at + from + len(` class="`)
	end := strings.Index(html[start:], `"`)
	require.GreaterOrEqual(t, end, 0, "unterminated class attribute on %q", id)
	return html[start : start+end]
}

// The opening zone is encoded in both the tab and the preview URL; a mismatch
// underlines Proof over a preview of the reveal.
func TestEditObjective_OpensTheTabThePreviewLoads(t *testing.T) {
	data := EditObjectiveData{
		Objective:    models.Objective{ID: "obj", Slug: "obj", Title: "Obj"},
		RevealBlocks: blocks.Blocks{blocks.NewMarkdownBlock(blocks.BaseBlock{})},
	}

	var out strings.Builder
	require.NoError(t, EditObjective(data).Render(context.Background(), &out))

	html := out.String()
	assert.Contains(t, classOf(t, html, "nav-tab"), "tab-active",
		"Reveal is the tab that opens")
	assert.NotContains(t, classOf(t, html, "content-tab"), "tab-active",
		"and Proof is not underlined over it")
	assert.Equal(t, 1, strings.Count(html, "?zone=proof"),
		"proof is named only by its own tab, not by the preview")
	assert.Equal(t, 2, strings.Count(html, "?zone=reveal"),
		"the Reveal tab names it, and so does the preview that loads on arrival")
}
