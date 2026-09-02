package admin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/migrations"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

func setupObjectiveTestDB(t *testing.T) (*bun.DB, func()) {
	t.Helper()
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	dbc := db.MustOpen(slog.New(slog.DiscardHandler))
	ctx := context.Background()

	migrator := migrate.NewMigrator(dbc, migrations.Migrations)
	require.NoError(t, migrator.Init(ctx))
	require.NoError(t, migrator.Lock(ctx))
	defer func() { require.NoError(t, migrator.Unlock(ctx)) }()
	_, err := migrator.Migrate(ctx)
	require.NoError(t, err)

	return dbc, func() { dbc.Close() }
}

// newObjectiveTestHandler wires a Handler with only the real services the
// Objective* handlers touch, backed by an in-memory DB: mirroring the
// integration-style pattern already used for services/players package tests
// rather than hand-rolled mocks.
func newObjectiveTestHandler(t *testing.T, dbc *bun.DB) *Handler {
	t.Helper()

	transactor := db.NewTransactor(dbc)
	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	uploadsRepo := repositories.NewUploadRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	logger := slog.New(slog.DiscardHandler)

	return &Handler{
		logger:           logger,
		objectiveService: services.NewObjectiveService(transactor, objectiveRepo),
		blockService:     services.NewBlockService(blockRepo, blockStateRepo),
		deleteService: services.NewDeleteService(
			transactor,
			instanceRepo,
			teamRepo,
			uploadsRepo,
			dbc,
			t.TempDir(),
			logger,
		),
		questService: services.NewQuestService(
			transactor,
			instanceRepo,
			instanceSettingsRepo,
			blockRepo,
			objectiveRepo,
		),
	}
}

// objectiveTestQuest creates a FK-valid user+quest and returns a *models.User
// with CurrentQuestID/CurrentQuest populated, matching what UserFromContext
// expects the session middleware to have set.
func objectiveTestQuest(t *testing.T, dbc *bun.DB) *models.User {
	t.Helper()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	quest := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "test-quest",
	}
	_, err = dbc.NewInsert().Model(quest).Exec(ctx)
	require.NoError(t, err)

	// Every quest has a root objective; the handler places new objectives
	// under it when no parent is named.
	root := &models.Objective{
		ID: gofakeit.UUID(), QuestID: quest.ID,
		Slug: "root", Title: "test-quest", Routing: models.RouteStrategyFreeRoam,
	}
	_, err = dbc.NewInsert().Model(root).Exec(ctx)
	require.NoError(t, err)

	user.CurrentQuestID = quest.ID
	user.CurrentQuest = *quest
	return user
}

func withObjectiveUser(r *http.Request, user *models.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextkeys.UserKey, user))
}

func withObjectiveSlug(r *http.Request, slug string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestObjectiveNew(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	req := httptest.NewRequest(http.MethodGet, "/admin/objective/new", nil)
	req.Header.Set("Hx-Request", "true")
	req = withObjectiveUser(req, user)

	w := httptest.NewRecorder()
	h.ObjectiveNew(w, req)

	location := w.Header().Get("Hx-Location")
	require.NotEmpty(t, location, "should redirect to the new objective's edit page")
	assert.True(t, strings.HasPrefix(location, "/admin/objective/"))

	slug := strings.TrimPrefix(location, "/admin/objective/")
	objective, err := h.objectiveService.GetByQuestIDAndSlug(context.Background(), user.CurrentQuestID, slug)
	require.NoError(t, err)
	assert.Equal(t, "New Objective", objective.Title)

	stored, err := h.objectiveService.GetByQuestIDAndSlug(context.Background(), user.CurrentQuestID, objective.Slug)
	require.NoError(t, err)
	root, err := h.objectiveService.FindRoot(context.Background(), user.CurrentQuestID)
	require.NoError(t, err)
	assert.Equal(t, root.ID, stored.ParentID, "a new objective lands under the root when no parent is named")
}

// A named parent is where the objective goes, appended after whatever is
// already there: a caller adding something new has no opinion about where among
// its siblings it lands.
func TestObjectiveNew_PlacedUnderNamedParent(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	ctx := context.Background()

	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)

	section, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, root.ID, "A section")
	require.NoError(t, err)

	existing, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, section.ID, "Existing")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/admin/objective/new", nil)
	req.URL.RawQuery = url.Values{"parentId": {section.ID}}.Encode()
	req = withObjectiveUser(req, user)

	w := httptest.NewRecorder()
	h.ObjectiveNew(w, req)

	children, err := h.objectiveService.FindChildren(ctx, user.CurrentQuestID, section.ID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	assert.Equal(t, existing.ID, children[0].ID)
	assert.Equal(t, []int{0, 1}, []int{children[0].Position, children[1].Position})
}

func TestObjectiveEdit(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	root, err := h.objectiveService.FindRoot(context.Background(), user.CurrentQuestID)
	require.NoError(t, err)

	t.Run("renders the edit page for an existing objective", func(t *testing.T) {
		objective, err := h.objectiveService.CreateObjective(
			context.Background(),
			user.CurrentQuestID,
			root.ID,
			"Find the key",
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/admin/objective/"+objective.Slug, nil)
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, objective.Slug)

		w := httptest.NewRecorder()
		h.ObjectiveEdit(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Find the key")
	})

	t.Run("unknown slug redirects to the quest list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/objective/does-not-exist", nil)
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, "does-not-exist")

		w := httptest.NewRecorder()
		h.ObjectiveEdit(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/admin/quest", w.Header().Get("Location"))
	})
}

// A real fault (as opposed to a genuine not-found) must not be silently
// treated as "this objective doesn't exist" and redirected away from: it
// should surface as an error instead.
func TestObjectiveEdit_RealErrorDoesNotRedirect(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	req := httptest.NewRequest(http.MethodGet, "/admin/objective/whatever", nil)
	req = withObjectiveUser(req, user)
	req = withObjectiveSlug(req, "whatever")

	cleanup() // close the DB so the lookup fails with something other than sql.ErrNoRows.

	w := httptest.NewRecorder()
	h.ObjectiveEdit(w, req)

	assert.NotEqual(t, http.StatusFound, w.Code, "a real error must not redirect like a not-found does")
	assert.Empty(t, w.Header().Get("Location"))
	assert.Contains(t, w.Body.String(), "Error finding objective")
}

func TestObjectiveEditPost(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	root, err := h.objectiveService.FindRoot(context.Background(), user.CurrentQuestID)
	require.NoError(t, err)
	t.Run("updates the title without a slug change", func(t *testing.T) {
		objective, err := h.objectiveService.CreateObjective(
			context.Background(),
			user.CurrentQuestID,
			root.ID,
			"Original Title",
		)
		require.NoError(t, err)

		form := url.Values{"title": {"Original Title"}}
		req := httptest.NewRequest(
			http.MethodPost,
			"/admin/objective/"+objective.Slug,
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, objective.Slug)

		w := httptest.NewRecorder()
		h.ObjectiveEditPost(w, req)

		assert.Empty(t, w.Header().Get("Hx-Redirect"), "no slug change means no redirect")

		reloaded, err := h.objectiveService.GetByQuestIDAndSlug(
			context.Background(),
			user.CurrentQuestID,
			objective.Slug,
		)
		require.NoError(t, err)
		assert.Equal(t, "Original Title", reloaded.Title)
	})

	t.Run("slug change redirects to the new path", func(t *testing.T) {
		root, rootErr := h.objectiveService.FindRoot(context.Background(), user.CurrentQuestID)
		require.NoError(t, rootErr)
		objective, err := h.objectiveService.CreateObjective(
			context.Background(), user.CurrentQuestID, root.ID, "Old Title")
		require.NoError(t, err)

		form := url.Values{"title": {"Brand New Title"}}
		req := httptest.NewRequest(
			http.MethodPost,
			"/admin/objective/"+objective.Slug,
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Hx-Request", "true")
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, objective.Slug)

		w := httptest.NewRecorder()
		h.ObjectiveEditPost(w, req)

		assert.Equal(t, "/admin/objective/brand-new-title", w.Header().Get("Hx-Redirect"))
	})
}

func TestObjectiveDelete(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	root, err := h.objectiveService.FindRoot(context.Background(), user.CurrentQuestID)
	require.NoError(t, err)

	t.Run("deletes an existing objective", func(t *testing.T) {
		objective, err := h.objectiveService.CreateObjective(
			context.Background(),
			user.CurrentQuestID,
			root.ID,
			"Find the key",
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/admin/objective/"+objective.Slug, nil)
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, objective.Slug)

		w := httptest.NewRecorder()
		h.ObjectiveDelete(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/admin/quest", w.Header().Get("Location"))

		_, err = h.objectiveService.GetByQuestIDAndSlug(context.Background(), user.CurrentQuestID, objective.Slug)
		assert.Error(t, err, "objective should no longer exist")
	})

	t.Run("unknown slug does not redirect to the quest list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/objective/does-not-exist", nil)
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, "does-not-exist")

		w := httptest.NewRecorder()
		h.ObjectiveDelete(w, req)

		assert.NotEqual(t, "/admin/quest", w.Header().Get("Location"))
	})
}
