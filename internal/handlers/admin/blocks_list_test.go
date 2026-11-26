package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// blockListFixture is two authors with an objective each, and a handler wired to
// check who may touch which block.
type blockListFixture struct {
	h    *Handler
	dbc  *bun.DB
	user *models.User
	// rootID is the user's objective, holding proof and reveal.
	rootID string
	proof  blocks.Block
	reveal blocks.Block
	// elsewhere is another objective of the same user, with a block of its own.
	elsewhere blocks.Block
	// foreign belongs to a different author altogether.
	foreignOwner string
	foreign      blocks.Block
}

func setupBlockList(t *testing.T) blockListFixture {
	t.Helper()
	dbc, cleanup := setupObjectiveTestDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	h := newObjectiveTestHandler(t, dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, repositories.NewBlockStateRepository(dbc))
	h.accessService = services.NewAccessService(blockRepo, repositories.NewQuestRepository(dbc), objectiveRepo)

	user := objectiveTestQuest(t, dbc)
	root, err := objectiveRepo.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)
	other := &models.Objective{
		ID: gofakeit.UUID(), QuestID: user.CurrentQuestID, Slug: "elsewhere", Title: "Elsewhere",
		Routing: models.RouteStrategyFreeRoam,
	}
	_, err = dbc.NewInsert().Model(other).Exec(ctx)
	require.NoError(t, err)

	stranger := objectiveTestQuest(t, dbc)
	strangerRoot, err := objectiveRepo.FindRoot(ctx, stranger.CurrentQuestID)
	require.NoError(t, err)

	build := func(owner string, zone blocks.BlockContext) blocks.Block {
		block, buildErr := h.blockService.NewBlockWithOwnerAndContext(ctx, owner, zone, "text")
		require.NoError(t, buildErr)
		return block
	}
	return blockListFixture{
		h: h, dbc: dbc, user: user, rootID: root.ID,
		proof:        build(root.ID, blocks.ContextObjectiveProof),
		reveal:       build(root.ID, blocks.ContextObjectiveReveal),
		elsewhere:    build(other.ID, blocks.ContextObjectiveProof),
		foreignOwner: strangerRoot.ID,
		foreign:      build(strangerRoot.ID, blocks.ContextObjectiveProof),
	}
}

// post sends a form to a handler as the fixture's user, with the block id in the
// route when there is one.
func (f blockListFixture) post(
	t *testing.T, handler func(http.ResponseWriter, *http.Request), blockID string, form url.Values,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/blocks/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withObjectiveUser(req, f.user)
	if blockID != "" {
		req = withBlockID(req, blockID)
	}
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func (f blockListFixture) idsIn(t *testing.T, owner string, zone blocks.BlockContext) []string {
	t.Helper()
	list, err := f.h.blockService.FindByOwnerIDAndContext(context.Background(), owner, zone)
	require.NoError(t, err)
	ids := make([]string, 0, len(list))
	for _, block := range list {
		ids = append(ids, block.GetID())
	}
	return ids
}

func kitForm(owner, zone, kit string) url.Values {
	form := url.Values{"owner": {owner}, "context": {zone}}
	if kit != "" {
		form.Set("kit", kit)
	}
	return form
}

func TestBlockKit_AddsTheTemplatesBlocks(t *testing.T) {
	f := setupBlockList(t)
	before := len(f.idsIn(t, f.rootID, blocks.ContextObjectiveProof))

	w := f.post(t, f.h.BlockKit, "", url.Values{
		"owner": {f.rootID}, "context": {"objective_proof"}, "kit": {"scan"},
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 2, strings.Count(w.Body.String(), "data-rail-for="), "a prompt and a scan, each with its row")
	assert.Len(t, f.idsIn(t, f.rootID, blocks.ContextObjectiveProof), before+2)
}

func TestBlockKit_FillsAnEmptyStartPage(t *testing.T) {
	f := setupBlockList(t)

	w := f.post(t, f.h.BlockKit, "", kitForm(f.user.CurrentQuestID, "start", "start_welcome"))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "data-rail-for=")
	assert.NotEmpty(t, f.idsIn(t, f.user.CurrentQuestID, blocks.ContextStart))
}

func TestBlockKit_RefusesWhatItCannotDo(t *testing.T) {
	f := setupBlockList(t)
	before := len(f.idsIn(t, f.rootID, blocks.ContextObjectiveProof))

	cases := []struct {
		name   string
		form   url.Values
		status int
	}{
		{"an unknown template", kitForm(f.rootID, "objective_proof", "nonsense"), 400},
		{"no template named", kitForm(f.rootID, "objective_proof", ""), 400},
		{"a page that is not an objective", kitForm(f.rootID, "start", "scan"), 400},
		{"another author's objective", kitForm(f.foreignOwner, "objective_proof", "scan"), 403},
		{"a page template on an objective", kitForm(f.rootID, "objective_proof", "start_welcome"), 400},
		{"a page template on the wrong page", kitForm(f.user.CurrentQuestID, "finish", "start_welcome"), 400},
		{"a finish template on the start page", kitForm(f.user.CurrentQuestID, "start", "finish_survey"), 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := f.post(t, f.h.BlockKit, "", c.form)
			assert.Equal(t, c.status, w.Code)
		})
	}
	assert.Len(t, f.idsIn(t, f.rootID, blocks.ContextObjectiveProof), before, "none of them made a block")
	assert.Empty(t, f.idsIn(t, f.foreignOwner, blocks.ContextObjectiveReveal), "nor in the other author's objective")
}

func TestBlockMove_MovesBetweenProofAndReveal(t *testing.T) {
	f := setupBlockList(t)

	w := f.post(t, f.h.BlockMove, f.proof.GetID(), url.Values{
		"context":  {"objective_reveal"},
		"block_id": {f.reveal.GetID(), f.proof.GetID()},
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{f.reveal.GetID(), f.proof.GetID()}, f.idsIn(t, f.rootID, blocks.ContextObjectiveReveal))
	assert.Empty(t, f.idsIn(t, f.rootID, blocks.ContextObjectiveProof))
}

func TestBlockMove_RefusesWhatItShouldNot(t *testing.T) {
	f := setupBlockList(t)
	proofBefore := f.idsIn(t, f.rootID, blocks.ContextObjectiveProof)
	revealBefore := f.idsIn(t, f.rootID, blocks.ContextObjectiveReveal)

	cases := []struct {
		name   string
		id     string
		form   url.Values
		status int
	}{
		{"onto a system page", f.proof.GetID(), url.Values{"context": {"start"}, "block_id": {f.proof.GetID()}}, 400},
		{
			"another author's block", f.foreign.GetID(),
			url.Values{"context": {"objective_reveal"}, "block_id": {f.foreign.GetID()}}, 403,
		},
		{
			"an order naming another objective's block", f.proof.GetID(),
			url.Values{"context": {"objective_reveal"}, "block_id": {f.elsewhere.GetID(), f.proof.GetID()}}, 400,
		},
		{
			"an order naming another author's block", f.proof.GetID(),
			url.Values{"context": {"objective_reveal"}, "block_id": {f.foreign.GetID(), f.proof.GetID()}}, 403,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := f.post(t, f.h.BlockMove, c.id, c.form)
			assert.Equal(t, c.status, w.Code)
		})
	}
	assert.Equal(t, proofBefore, f.idsIn(t, f.rootID, blocks.ContextObjectiveProof), "nothing moved")
	assert.Equal(t, revealBefore, f.idsIn(t, f.rootID, blocks.ContextObjectiveReveal))
	assert.Equal(t, []string{f.foreign.GetID()}, f.idsIn(t, f.foreignOwner, blocks.ContextObjectiveProof))
}

func TestBlockReorder_RefusesAnotherAuthorsBlock(t *testing.T) {
	f := setupBlockList(t)

	w := f.post(t, f.h.BlockReorder, "", url.Values{"block_id": {f.foreign.GetID(), f.proof.GetID()}})

	assert.Equal(t, http.StatusForbidden, w.Code, "a script has to be able to tell it was refused")
}

func TestBlockCreate_AnswersABlockThePageCannotHoldWith400(t *testing.T) {
	f := setupBlockList(t)

	// A game status is a start page block.
	w := f.post(t, f.h.BlockCreate, "", url.Values{
		"owner": {f.rootID}, "context": {"objective_proof"}, "type": {"game_status"},
	})

	assert.Equal(t, http.StatusBadRequest, w.Code, "the request's fault, not the server's")
}
