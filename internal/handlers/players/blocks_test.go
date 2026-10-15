package players_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/middlewares"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// previewQuests stands in for the quest the editor is previewing. A template
// skips the ownership check, so the identity service is never asked.
type previewQuests struct {
	questID string
}

func (p previewQuests) GetByID(context.Context, string) (*models.Quest, error) {
	return &models.Quest{ID: p.questID, IsTemplate: true}, nil
}

func (p previewQuests) GetQuestSettings(context.Context, string) (*models.QuestSettings, error) {
	return &models.QuestSettings{QuestID: p.questID}, nil
}

// A submission from the editor's preview pane runs the same endpoint players
// post to, and it must read as play without being recorded as any: it carries
// the synthetic "preview" run, which has no row in runs, so a single write
// would either trip a foreign key or land in the stream as an answer nobody
// gave. The chain under test is the route's own: PreviewMiddleware, then the
// two middlewares that stand aside for a preview, then the handler.
func TestPlayerHandler_ValidateBlock_PreviewSubmissionWritesNothing(t *testing.T) {
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

	block, err := blockService.NewBlockWithOwnerAndContext(
		ctx, objective.ID, blocks.ContextObjectiveProof, "free_text",
	)
	require.NoError(t, err)

	runService := runServiceFor(dbc)

	handler := players.NewTestPlayerHandler(
		players.WithCheckInService(checkInService),
		players.WithBlockService(blockService),
	)

	chain := middlewares.PreviewMiddleware(
		slog.Default(),
		runService,
		previewQuests{questID: parents.QuestID},
		nil,
		middlewares.RunMiddleware(slog.Default(), runService,
			middlewares.StartMiddleware(runService, http.HandlerFunc(handler.ValidateBlock))),
	)

	// The form a preview carries: the answer, and the quest the middleware
	// reads the preview's identity from.
	submit := func() *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{
			"questID":  {parents.QuestID},
			"block":    {block.GetID()},
			"response": {"the answer"},
		}
		req := httptest.NewRequest(http.MethodPost, "/blocks/validate", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// isPreviewRequest wants htmx, and a referer inside the editor.
		req.Header.Set("Hx-Request", "true")
		req.Header.Set("Referer", "https://example.test/admin/quest")

		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)
		return w
	}

	w := submit()
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "the answer",
		"the answer is validated and rendered, so the submission took the preview path")

	// Nothing of any write class, and nothing in the stream. The counts are
	// unfiltered: this fixture holds no other run, so anything at all is the
	// preview's doing.
	states, err := dbc.NewSelect().Model((*models.RunBlockState)(nil)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, states, "a preview keeps no block state")

	completions, err := dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, completions, "a preview completes no objective")

	events, err := dbc.NewSelect().Model((*models.RunEvent)(nil)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, events, "and records nothing in the stream")

	// Nothing was persisted, so the editor can try the same answer again. A
	// second submission that went quiet would be the shape of a stored
	// completion, which is the thing the first assertion says is absent.
	again := submit()
	assert.Equal(t, http.StatusOK, again.Code)
	assert.Contains(t, again.Body.String(), "the answer")
}

// runServiceFor wires a real RunService against the test's database, so the
// middlewares in the chain are the production ones rather than nil stand-ins.
func runServiceFor(dbc *bun.DB) *services.RunService {
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	runRepo := repositories.NewRunRepository(dbc)
	transactor := db.NewTransactor(dbc)

	creditService := services.NewCreditService(
		transactor,
		repositories.NewCreditRepository(dbc),
		repositories.NewRunStartLogRepository(dbc),
		nil,
	)

	return services.NewRunService(
		transactor,
		runRepo,
		creditService,
		blockStateRepo,
		repositories.NewObjectiveRepository(dbc),
		repositories.NewObjectiveContextCompletionRepository(dbc),
		repositories.NewSectionFinishRepository(dbc),
		blockRepo,
		repositories.NewRunEventRepository(dbc),
	)
}
