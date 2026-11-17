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

func TestPlayerHandler_ValidateBlock_LastAnswerOffersAToastNotTheReveal(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := createTestRun(t, dbc, parents.QuestID)
	team := &models.Run{Code: runCode, QuestID: parents.QuestID}

	root := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "root", Title: "Root"}
	task := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: root.ID, Slug: "garden", Title: "The garden",
	}
	for _, o := range []*models.Objective{root, task} {
		_, err := dbc.NewInsert().Model(o).Exec(ctx)
		require.NoError(t, err)
	}

	answer, err := blockService.NewBlockWithOwnerAndContext(ctx, task.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)
	reveal, err := blockService.NewBlockWithOwnerAndContext(ctx, task.ID, blocks.ContextObjectiveReveal, "text")
	require.NoError(t, err)
	_, err = blockService.UpdateBlock(ctx, reveal, map[string][]string{"content": {"The vault code is 4417"}})
	require.NoError(t, err)
	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, task.ID, runCode, parents.QuestID, blocks.ContextObjectiveProof,
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
	assert.Contains(t, body, `id="reveal-toast"`)
	assert.Contains(t, body, `hx-get="/objective/garden"`)
	assert.NotContains(t, body, "4417", "the reveal is fetched after the beat, not pushed in now")
}
