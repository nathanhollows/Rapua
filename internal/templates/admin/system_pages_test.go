package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderSystemPage(t *testing.T, pageType, title string, list blocks.Blocks) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, SystemPageEditor(services.QuestLint{}, pageType, EditPageData{
		Settings:   models.QuestSettings{QuestID: "quest-1"},
		PageBlocks: list,
		PageTitle:  title,
		PageType:   pageType,
	}).Render(context.Background(), &out))
	return out.String()
}

// The start and complete pages are edited in the same block editor as an
// objective's proof and reveal, not in the accordion they had before.
func TestSystemPageEditor_UsesTheBlockEditor(t *testing.T) {
	html := renderSystemPage(t, "start", "Start", blocks.Blocks{
		blocks.NewMarkdownBlock(blocks.BaseBlock{ID: "text-1", OwnerID: "quest-1"}),
	})

	assert.Contains(t, html, `data-objective-editor`)
	assert.Contains(t, html, `id="rail-start"`)
	assert.Contains(t, html, `data-block-form="text-1"`)
	assert.Contains(t, html, `data-owner="quest-1"`, "blocks belong to the quest, not an objective")
	assert.Contains(t, html, `hx-get="/start"`, "its phone is the player's start page")
	assert.Equal(t, 1, strings.Count(html, `class="cf-phone"`))
	assert.NotContains(t, html, "mockup-phone")
	assert.NotContains(t, html, "collapse")
	assert.Contains(t, html, `<h1 class="text-4xl font-black mr-auto">Start page</h1>`)
}

func TestSystemPageEditor_CompletePreviewsTheCompletePage(t *testing.T) {
	html := renderSystemPage(t, "complete", "Complete", nil)

	assert.Contains(t, html, `id="rail-finish"`)
	assert.Contains(t, html, `hx-get="/complete"`)
	assert.Contains(t, html, `Complete page</h1>`)
	assert.Contains(t, html, "Nothing on this page yet", "an empty page says so and offers a block")
}

// The page recommends blocks it lacks; the script shows each hint only while
// no block of its type is in the list, so the list rows have to carry the type.
func TestSystemPageEditor_HintsNameTheBlocksTheyRecommend(t *testing.T) {
	start := renderSystemPage(t, "start", "Start", nil)
	assert.Contains(t, start, `data-hint-for="game_status"`)
	assert.Contains(t, start, `data-hint-for="start_button"`)
	assert.Contains(t, start, `hx-target="#forms-start"`, "an added block lands in the page's own forms")

	// The server refuses a game status on the complete page, so it is not offered.
	complete := renderSystemPage(t, "complete", "Complete", nil)
	assert.NotContains(t, complete, "data-hint-for")

	withButton := renderSystemPage(t, "start", "Start", blocks.Blocks{
		blocks.NewStartGameButtonBlock(blocks.BaseBlock{ID: "go", OwnerID: "quest-1"}),
	})
	assert.Contains(t, withButton, `data-type="start_button"`)
}

// The phone view draws the player's dock where the real screen has one. The
// start page is pre-game, and the player's own layout gives it none.
func TestSystemPageEditor_DockFollowsThePlayersLayout(t *testing.T) {
	assert.NotContains(t, renderSystemPage(t, "start", "Start", nil), "cf-dock")
	assert.Equal(t, 1, strings.Count(renderSystemPage(t, "complete", "Complete", nil), `class="cf-dock player"`))
}
