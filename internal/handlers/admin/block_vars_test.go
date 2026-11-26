package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withBlockID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// blockVarsFixture is a quest with one objective holding an answer that sets a
// variable and a text block, plus a handler able to edit both.
type blockVarsFixture struct {
	h      *Handler
	user   *models.User
	answer blocks.Block
	text   blocks.Block
}

func setupBlockVars(t *testing.T) blockVarsFixture {
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

	answer, err := h.blockService.NewBlockWithOwnerAndContext(ctx, root.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)
	sets := "met_guide"
	answer, err = h.blockService.UpdateBlockVars(ctx, answer, &sets, nil)
	require.NoError(t, err)
	text, err := h.blockService.NewBlockWithOwnerAndContext(ctx, root.ID, blocks.ContextObjectiveReveal, "text")
	require.NoError(t, err)

	return blockVarsFixture{h: h, user: user, answer: answer, text: text}
}

func (f blockVarsFixture) do(t *testing.T, method string, block blocks.Block, form url.Values) string {
	t.Helper()
	req := httptest.NewRequest(method, "/admin/blocks/"+block.GetID()+"/vars", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withBlockID(withObjectiveUser(req, f.user), block.GetID())
	w := httptest.NewRecorder()
	if method == http.MethodGet {
		f.h.BlockVarsEdit(w, req)
	} else {
		f.h.BlockVarsUpdate(w, req)
	}
	require.Equal(t, http.StatusOK, w.Code)
	return w.Body.String()
}

func TestBlockVarsEdit_OffersTheQuestsVariables(t *testing.T) {
	f := setupBlockVars(t)

	body := f.do(t, http.MethodGet, f.text, nil)
	assert.Contains(t, body, `name="depends"`)
	assert.Contains(t, body, `value="met_guide"`, "the lock can be set to what the answer sets")

	body = f.do(t, http.MethodGet, f.answer, nil)
	assert.Contains(t, body, `name="sets"`)
	assert.Contains(t, body, `<datalist`)
}

func TestBlockVarsUpdate_SavesAndFoldsBackIntoTheButton(t *testing.T) {
	f := setupBlockVars(t)

	body := f.do(t, http.MethodPut, f.text, url.Values{"depends": {"", "met_guide"}})
	assert.Contains(t, body, `id="block-vars-btn-`+f.text.GetID()+`"`)
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, "met_guide")

	saved, err := f.h.blockService.GetByBlockID(context.Background(), f.text.GetID())
	require.NoError(t, err)
	assert.Equal(t, []string{"met_guide"}, []string(saved.GetDepends()))

	// The blank the form always sends is how unticking everything clears it.
	f.do(t, http.MethodPut, f.text, url.Values{"depends": {""}})
	saved, err = f.h.blockService.GetByBlockID(context.Background(), f.text.GetID())
	require.NoError(t, err)
	assert.Empty(t, saved.GetDepends())
}

func TestBlockVarsUpdate_RefusesWhatTheBlockCannotCarry(t *testing.T) {
	f := setupBlockVars(t)

	f.do(t, http.MethodPut, f.text, url.Values{"sets": {"something"}})

	saved, err := f.h.blockService.GetByBlockID(context.Background(), f.text.GetID())
	require.NoError(t, err)
	assert.Empty(t, saved.GetSets(), "a content block never sets a variable")
}

func TestObjectiveDependsPost_SavesAndRefusesTheRoot(t *testing.T) {
	f := setupBlockVars(t)
	ctx := context.Background()

	root, err := f.h.objectiveService.FindRoot(ctx, f.user.CurrentQuestID)
	require.NoError(t, err)
	door, err := f.h.objectiveService.CreateObjective(ctx, f.user.CurrentQuestID, root.ID, "Door")
	require.NoError(t, err)

	post := func(slug string, form url.Values) string {
		req := httptest.NewRequest(
			http.MethodPost,
			"/admin/objective/"+slug+"/depends",
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = withObjectiveSlug(withObjectiveUser(req, f.user), slug)
		w := httptest.NewRecorder()
		f.h.ObjectiveDependsPost(w, req)
		return w.Body.String()
	}

	body := post(door.Slug, url.Values{"depends": {"", "met_guide"}})
	assert.Contains(t, body, `id="objective-depends-btn"`)
	assert.Contains(t, body, "met_guide")
	saved, err := f.h.objectiveService.GetByQuestIDAndSlug(ctx, f.user.CurrentQuestID, door.Slug)
	require.NoError(t, err)
	assert.Equal(t, []string{"met_guide"}, []string(saved.Depends))

	post(root.Slug, url.Values{"depends": {"met_guide"}})
	reloaded, err := f.h.objectiveService.FindRoot(ctx, f.user.CurrentQuestID)
	require.NoError(t, err)
	assert.Empty(t, reloaded.Depends, "the root is the quest and always open")
}
