package players_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An answer that sets a variable brings the blocks waiting on it onto the page
// it was given on, without the player reloading.
func TestPlayerHandler_ValidateBlock_SwapsInWhatTheAnswerUnlocked(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := createTestRun(t, dbc, parents.QuestID)
	team := &models.Run{Code: runCode, QuestID: parents.QuestID}

	objective := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "lock", Title: "The lock"}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	newBlock := func(blockType string) blocks.Block {
		b, newErr := blockService.NewBlockWithOwnerAndContext(
			ctx,
			objective.ID,
			blocks.ContextObjectiveProof,
			blockType,
		)
		require.NoError(t, newErr)
		return b
	}
	answer := newBlock("free_text")
	answer.SetVars("met_guide", nil)
	_, err = blockService.UpdateBlock(ctx, answer, map[string][]string{})
	require.NoError(t, err)

	secret := newBlock("text")
	secret.SetVars("", []string{"met_guide"})
	_, err = blockService.UpdateBlock(ctx, secret, map[string][]string{"content": {"The code is 4417"}})
	require.NoError(t, err)

	// A second question keeps the proof open, so the page stays and the
	// unlocked block has to arrive on its own.
	newBlock("free_text")

	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, objective.ID, runCode, parents.QuestID, blocks.ContextObjectiveProof,
	)
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)
	form := url.Values{"block": {answer.GetID()}, "response": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/blocks/validate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), contextkeys.RunKey, team))
	w := httptest.NewRecorder()

	handler.ValidateBlock(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `id="preview-block-`+secret.GetID()+`"`)
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, "4417")
}

// A block answered on an objective the run cannot reach saves its state, but
// the reveal it would clear stays hidden.
func TestPlayerHandler_ValidateBlock_UnreachableRevealStaysHidden(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := createTestRun(t, dbc, parents.QuestID)
	team := &models.Run{Code: runCode, QuestID: parents.QuestID}

	root := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "root", Title: "Root"}
	door := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: root.ID, Slug: "door", Title: "Door",
		Depends: []string{"never_set"},
	}
	for _, o := range []*models.Objective{root, door} {
		_, err := dbc.NewInsert().Model(o).Exec(ctx)
		require.NoError(t, err)
	}

	answer, err := blockService.NewBlockWithOwnerAndContext(ctx, door.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)
	reveal, err := blockService.NewBlockWithOwnerAndContext(ctx, door.ID, blocks.ContextObjectiveReveal, "text")
	require.NoError(t, err)
	_, err = blockService.UpdateBlock(ctx, reveal, map[string][]string{"content": {"The vault code is 4417"}})
	require.NoError(t, err)
	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, door.ID, runCode, parents.QuestID, blocks.ContextObjectiveProof,
	)
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)
	form := url.Values{"block": {answer.GetID()}, "response": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/blocks/validate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), contextkeys.RunKey, team))
	w := httptest.NewRecorder()

	handler.ValidateBlock(w, req)

	assert.NotContains(t, w.Body.String(), "4417")
}
