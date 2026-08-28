package players_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupObjectivePreviewServices wires real CheckInService/BlockService against
// an in-memory DB, mirroring setupCheckInServiceForObjectives in the services
// package: objectivePreview never touches locationStatsService/navigationService.
func setupObjectivePreviewServices(t *testing.T) (*services.CheckInService, *services.BlockService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)

	checkInService := services.NewCheckInService(
		repositories.NewCheckInRepository(dbc),
		repositories.NewLocationRepository(dbc),
		repositories.NewRunRepository(dbc),
		nil, nil,
		blockService,
		repositories.NewRunVarStateRepository(dbc),
		repositories.NewObjectiveRepository(dbc),
		repositories.NewObjectiveContextCompletionRepository(dbc),
	)
	return checkInService, blockService, dbc, cleanup
}

// requestForObjectivePreview builds a preview request carrying the synthetic
// preview run (as PreviewMiddleware would inject) and the chi "slug" param.
func requestForObjectivePreview(slug string, team *models.Run, zone string) *http.Request {
	target := "/objective/" + slug
	if zone != "" {
		target += "?zone=" + zone
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, contextkeys.RunKey, team)
	ctx = context.WithValue(ctx, contextkeys.PreviewKey, true)

	return req.WithContext(ctx)
}

func TestPlayerHandler_ObjectiveView_Preview_DefaultsToProof(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	_, err = blockService.NewBlockWithOwnerAndContext(ctx, objective.ID, "objective_proof", "text")
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)

	team := &models.Run{
		Code:    "preview",
		QuestID: parents.QuestID,
		Quest:   models.Quest{ID: parents.QuestID},
	}

	w := httptest.NewRecorder()
	handler.ObjectiveView(w, requestForObjectivePreview(objective.Slug, team, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "block-view", "proof block should render in the fragment")
}

func TestPlayerHandler_ObjectiveView_Preview_RevealZone(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)

	team := &models.Run{
		Code:    "preview",
		QuestID: parents.QuestID,
		Quest:   models.Quest{ID: parents.QuestID},
	}

	w := httptest.NewRecorder()
	handler.ObjectiveView(w, requestForObjectivePreview(objective.Slug, team, "reveal"))

	assert.Equal(t, http.StatusOK, w.Code)
}

// Preview must never write real completion state: no runs row exists for
// "preview", so a call into CompleteObjectiveContext (which the real,
// non-preview path uses) would violate the run_code FK. This proves the
// preview render path never reaches it, independent of the guard added
// directly to CompleteObjectiveContext.
func TestPlayerHandler_ObjectiveView_Preview_DoesNotWriteCompletionState(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)

	team := &models.Run{
		Code:    "preview",
		QuestID: parents.QuestID,
		Quest:   models.Quest{ID: parents.QuestID},
	}

	w := httptest.NewRecorder()
	handler.ObjectiveView(w, requestForObjectivePreview(objective.Slug, team, "proof"))
	require.Equal(t, http.StatusOK, w.Code)

	count, err := dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ?", objective.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "preview must not log a completion")
}

func TestPlayerHandler_ObjectiveView_Preview_UnknownSlug(t *testing.T) {
	checkInService, blockService, dbc, cleanup := setupObjectivePreviewServices(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)

	team := &models.Run{
		Code:    "preview",
		QuestID: parents.QuestID,
		Quest:   models.Quest{ID: parents.QuestID},
	}

	w := httptest.NewRecorder()
	handler.ObjectiveView(w, requestForObjectivePreview("does-not-exist", team, ""))

	// handleError renders an error toast rather than setting a non-200 status.
	assert.Contains(t, w.Body.String(), "Objective not found")
}
