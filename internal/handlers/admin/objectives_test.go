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
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/migrations"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
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
			uploadsRepo, objectiveRepo,
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
		lintService: services.NewLintService(
			instanceRepo,
			instanceSettingsRepo,
			objectiveRepo,
			blockRepo,
			blocks.Registry(),
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

	// Settings are not optional to the handlers under test: lint loads them,
	// and a fixture without them sends every test down an error path where the
	// success path renders nothing and asserts nothing.
	_, err = dbc.NewInsert().Model(&models.QuestSettings{QuestID: quest.ID}).Exec(ctx)
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

	// A rename used to move the objective's address, so the handler had to send
	// the author to the new one. That redirect is a full page load, and a full
	// page load drops the unlock on a running quest: the autosave appeared to
	// relock the page mid-edit. With the address fixed there is nowhere to go.
	t.Run("renaming does not move the author", func(t *testing.T) {
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

		assert.Empty(t, w.Header().Get("Hx-Redirect"))

		// Still reachable where it was, and renamed.
		reloaded, err := h.objectiveService.GetByQuestIDAndSlug(
			context.Background(), user.CurrentQuestID, objective.Slug)
		require.NoError(t, err)
		assert.Equal(t, "Brand New Title", reloaded.Title)
	})
}

// A rename must not change the slug or redirect: the redirect reloads the page,
// which re-ran questLock.set(true) and relocked a quest the author had just
// unlocked. Both are asserted because either alone passes the wrong way.
func TestObjectiveEditPost_RenamingDoesNotMoveTheObjective(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	root, err := h.objectiveService.FindRoot(context.Background(), user.CurrentQuestID)
	require.NoError(t, err)
	objective, err := h.objectiveService.CreateObjective(
		context.Background(), user.CurrentQuestID, root.ID, "Rose",
	)
	require.NoError(t, err)
	before := objective.Slug

	form := url.Values{"title": {"Scan the label at the rose bed"}}
	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/objective/"+before,
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withObjectiveUser(req, user)
	req = withObjectiveSlug(req, before)

	w := httptest.NewRecorder()
	h.ObjectiveEditPost(w, req)

	assert.Empty(t, w.Header().Get("Hx-Redirect"), "a rename must not send the page anywhere")
	assert.NotEqual(t, http.StatusFound, w.Code, "nor redirect the hard way")

	reloaded, err := h.objectiveService.GetByQuestIDAndSlug(
		context.Background(), user.CurrentQuestID, before,
	)
	require.NoError(t, err, "the old link still resolves")
	assert.Equal(t, "Scan the label at the rose bed", reloaded.Title)
	assert.Equal(t, before, reloaded.Slug, "the link is the objective's, not the title's")
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

// The builder renders the whole tree, drafts included, so it has to say which
// rows players cannot see. A row hidden only because an ancestor is drafted
// carries no flag of its own, and is the case an operator would otherwise
// edit, reorder or delete blind.
func TestObjectiveTreeNodes_MarksWhatIsOutOfPlay(t *testing.T) {
	root := models.Objective{ID: "root", Slug: "root", Title: "Root"}
	live := models.Objective{ID: "live", ParentID: "root", Slug: "live", Title: "Live"}
	parked := models.Objective{ID: "parked", ParentID: "root", Slug: "parked", Title: "Parked", Draft: true}
	buried := models.Objective{ID: "buried", ParentID: "parked", Slug: "buried", Title: "Buried"}

	_, nodes := buildObjectiveTree([]models.Objective{root, live, parked, buried}, nil, services.QuestLint{})

	bySlug := map[string]*templates.ObjectiveTreeNode{}
	for _, node := range nodes {
		bySlug[node.Objective.Slug] = node
	}
	require.Len(t, nodes, 2, "the root holds everything, so listing it says nothing")

	assert.False(t, bySlug["live"].OutOfPlay)
	assert.True(t, bySlug["parked"].Draft)
	assert.True(t, bySlug["parked"].OutOfPlay)
	require.Len(t, bySlug["parked"].Children, 1, "buried nests under its parked parent")
	buriedNode := bySlug["parked"].Children[0]
	assert.False(t, buriedNode.Draft, "the flag stays where the author put it")
	assert.True(t, buriedNode.OutOfPlay, "but the builder still shows it as hidden")
}

// The visibility toggle is a checkbox, and an unticked checkbox is absent from
// the form rather than false. Its absence has to read as "park this", or an
// author could never hide anything.
func TestObjectiveEditPost_Visibility(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	ctx := context.Background()

	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)

	objective, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, root.ID, "Parked")
	require.NoError(t, err)
	require.True(t, objective.Draft, "new objectives arrive parked")

	post := func(form url.Values) {
		req := httptest.NewRequest(
			http.MethodPost, "/admin/objective/"+objective.Slug, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, objective.Slug)
		h.ObjectiveEditPost(httptest.NewRecorder(), req)
	}

	post(url.Values{"title": {"Parked"}, "published": {"true"}})
	reloaded, err := h.objectiveService.GetByQuestIDAndSlug(ctx, user.CurrentQuestID, objective.Slug)
	require.NoError(t, err)
	assert.False(t, reloaded.Draft, "ticking it publishes")

	post(url.Values{"title": {"Parked"}})
	reloaded, err = h.objectiveService.GetByQuestIDAndSlug(ctx, user.CurrentQuestID, objective.Slug)
	require.NoError(t, err)
	assert.True(t, reloaded.Draft, "and an absent checkbox parks it again")
}

// A refused move must leave the tree as it was. The handler answers 200 even
// on refusal (the toast contract), so the test reloads the order instead of
// trusting the status code.
func TestObjectiveReposition_MalformedPositionIsRefused(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	ctx := context.Background()

	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)
	first, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, root.ID, "First")
	require.NoError(t, err)
	second, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, root.ID, "Second")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/objective/reposition", strings.NewReader(
		url.Values{
			"objective_id": {first.ID},
			"parent_id":    {root.ID},
			"position":     {"not-a-number"},
		}.Encode(),
	))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withObjectiveUser(req, user)
	h.ObjectiveReposition(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "the toast contract answers 200 even on refusal")

	children, err := h.objectiveService.FindChildren(ctx, user.CurrentQuestID, root.ID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	assert.Equal(t, first.ID, children[0].ID, "the malformed move left the order untouched")
	assert.Equal(t, second.ID, children[1].ID)
}

// The fixture has to reach lint's success path. It did not, and that silence
// was what let four defects through: every assertion about the builder page
// was made against a render that had already given up.
func TestObjectiveTestQuest_ReachesLint(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()

	handler := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)

	_, err := handler.lintService.LintQuest(context.Background(), user.CurrentQuestID)
	require.NoError(t, err)
}

// A quest with two parentless rows is a state lint reports, so the tree has to
// draw both: only one can be the root, and the other was dropped along with
// everything under it, leaving a diagnostic about a row nobody could see.
func TestBuildObjectiveTree_SecondRootIsStillDrawn(t *testing.T) {
	first := models.Objective{ID: "first", Slug: "first", Title: "First"}
	second := models.Objective{ID: "second", Slug: "second", Title: "Second"}
	child := models.Objective{ID: "child", ParentID: "first", Slug: "child", Title: "Child"}

	root, nodes := buildObjectiveTree(
		[]models.Objective{first, second, child}, nil, services.QuestLint{})

	drawn := map[string]bool{}
	for _, node := range nodes {
		drawn[node.Objective.ID] = true
	}
	assert.NotEmpty(t, root.ID, "one of them has to be the root")

	other := "first"
	if root.ID == "first" {
		other = "second"
	}
	assert.True(t, drawn[other],
		"the parentless row that is not the root must still have a place to be dragged from")
}

// The band picker and the save guard have to count the same children. They did
// not: the picker offered every child and the guard accepted only the
// published ones, so "require all 2" on a section with a parked child was
// offered by the UI and refused by the save.
func TestBuildObjectiveTree_PublishedChildCountExcludesDrafts(t *testing.T) {
	root := models.Objective{ID: "root", Slug: "root", Title: "Root"}
	section := models.Objective{ID: "section", ParentID: "root", Slug: "section", Title: "Section"}
	live := models.Objective{ID: "live", ParentID: "section", Slug: "live", Title: "Live"}
	parked := models.Objective{ID: "parked", ParentID: "section", Slug: "parked", Title: "Parked", Draft: true}

	_, nodes := buildObjectiveTree(
		[]models.Objective{root, section, live, parked}, nil, services.QuestLint{})

	require.Len(t, nodes, 1)
	assert.Len(t, nodes[0].Children, 2, "both children are drawn")
	assert.Equal(t, 1, nodes[0].PublishedChildren, "but only one counts toward the band")
}

// A row in a parent cycle is attached to something but reachable from nothing,
// so the tree never drew it: lint named a row nobody could see or drag.
//
// Rendering, not just counting: the first version of this test asserted on the
// slice and passed while the page it produced never returned.
func TestBuildObjectiveTree_CycleRendersAndTerminates(t *testing.T) {
	root := models.Objective{ID: "root", Slug: "root", Title: "Root"}
	// first and second point at each other, so neither hangs off the root.
	first := models.Objective{ID: "first", ParentID: "second", Slug: "first", Title: "First"}
	second := models.Objective{ID: "second", ParentID: "first", Slug: "second", Title: "Second"}

	rootRow, nodes := buildObjectiveTree(
		[]models.Objective{root, first, second}, nil, services.QuestLint{})

	var out strings.Builder
	require.NoError(t, templates.ObjectiveTree(
		services.QuestLint{}, rootRow, nodes, false).Render(context.Background(), &out))

	rendered := out.String()
	assert.Equal(t, 1, strings.Count(rendered, `data-objective-id="first"`),
		"each row is drawn once")
	assert.Equal(t, 1, strings.Count(rendered, `data-objective-id="second"`),
		"including the one whose parent edge was cut")
}

// An orphan's children are unreachable too, so appending every unreachable row
// drew the same card once at the top level and again nested under its parent.
func TestBuildObjectiveTree_OrphanChainIsNotDuplicated(t *testing.T) {
	root := models.Objective{ID: "root", Slug: "root", Title: "Root"}
	orphan := models.Objective{ID: "orphan", ParentID: "gone", Slug: "orphan", Title: "Orphan"}
	middle := models.Objective{ID: "middle", ParentID: "orphan", Slug: "middle", Title: "Middle"}
	leaf := models.Objective{ID: "leaf", ParentID: "middle", Slug: "leaf", Title: "Leaf"}

	rootRow, nodes := buildObjectiveTree(
		[]models.Objective{root, orphan, middle, leaf}, nil, services.QuestLint{})

	var out strings.Builder
	require.NoError(t, templates.ObjectiveTree(
		services.QuestLint{}, rootRow, nodes, false).Render(context.Background(), &out))

	rendered := out.String()
	for _, id := range []string{"orphan", "middle", "leaf"} {
		assert.Equal(t, 1, strings.Count(rendered, `data-objective-id="`+id+`"`),
			id+" is drawn once, nested where it belongs")
	}
}

// A row whose parent is missing is still drafted or not. The attach loop set
// that flag only on rows it could attach, so a parked orphan was drawn as
// though players could see it.
func TestBuildObjectiveTree_OrphanKeepsItsDraftState(t *testing.T) {
	root := models.Objective{ID: "root", Slug: "root", Title: "Root"}
	parked := models.Objective{
		ID: "parked", ParentID: "gone", Slug: "parked", Title: "Parked", Draft: true,
	}

	_, nodes := buildObjectiveTree(
		[]models.Objective{root, parked}, nil, services.QuestLint{})

	require.Len(t, nodes, 1)
	assert.True(t, nodes[0].Draft, "the row's own flag survives losing its parent")
	assert.True(t, nodes[0].OutOfPlay, "and it is drawn as out of play")
}

// The visibility button flips before the server has agreed. A refused park
// used to leave it flipped, so every later autosave resubmitted the refusal
// and the page quietly stopped saving anything.
func TestObjectiveEditPost_RefusedParkRestoresVisibility(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	ctx := context.Background()

	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)

	// A section needing its one published child, so parking that child is
	// refused.
	section, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, root.ID, "Section")
	require.NoError(t, err)
	child, err := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, section.ID, "Child")
	require.NoError(t, err)
	published := false
	require.NoError(t, h.objectiveService.UpdateObjective(ctx, &child,
		services.ObjectiveUpdateData{Draft: &published}))
	require.NoError(t, h.objectiveService.UpdateObjective(ctx, &section,
		services.ObjectiveUpdateData{Band: &services.BandUpdate{Min: intPtr(1)}}))

	// Park the child: the section could no longer reach its minimum.
	form := url.Values{"title": {"Child"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/objective/"+child.Slug, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withObjectiveUser(req, user)
	req = withObjectiveSlug(req, child.Slug)

	w := httptest.NewRecorder()
	h.ObjectiveEditPost(w, req)

	assert.Contains(t, w.Body.String(), `id="visibility-control"`,
		"the refusal sends the control back")
	assert.Contains(t, w.Body.String(), `hx-swap-oob`,
		"as an out-of-band swap, so the form stops offering the rejected state")
}

// Unreachable rows arrive in query order, so a stranded row's subtree was
// assembled before anyone knew the row itself was parked, and its children
// rendered as though players could see them.
func TestBuildObjectiveTree_ParkedOrphanDimsItsSubtree(t *testing.T) {
	root := models.Objective{ID: "root", Slug: "root", Title: "Root"}
	// The child is listed first, so the attach loop meets it before its parent.
	child := models.Objective{ID: "child", ParentID: "parked", Slug: "child", Title: "Child"}
	parked := models.Objective{
		ID: "parked", ParentID: "gone", Slug: "parked", Title: "Parked", Draft: true,
	}

	_, nodes := buildObjectiveTree(
		[]models.Objective{root, child, parked}, nil, services.QuestLint{})

	require.Len(t, nodes, 1)
	require.Len(t, nodes[0].Children, 1)
	assert.True(t, nodes[0].OutOfPlay, "the stranded row is parked")
	assert.True(t, nodes[0].Children[0].OutOfPlay, "and so is everything under it")
}

// A control the form never offered must not read as an instruction to clear
// it. The root toolbar has routing but no band inputs, and synthesising blanks
// for the band wiped the quest's own completion band on every routing change.
func TestObjectiveSettingsPost_AbsentControlsLeaveTheirSettingsAlone(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	ctx := context.Background()

	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)

	published := false
	for _, title := range []string{"One", "Two"} {
		child, childErr := h.objectiveService.CreateObjective(ctx, user.CurrentQuestID, root.ID, title)
		require.NoError(t, childErr)
		require.NoError(t, h.objectiveService.UpdateObjective(ctx, &child,
			services.ObjectiveUpdateData{Draft: &published}))
	}
	require.NoError(t, h.objectiveService.UpdateObjective(ctx, root,
		services.ObjectiveUpdateData{Band: &services.BandUpdate{Min: intPtr(1), Max: intPtr(2)}}))

	// Exactly what the root toolbar submits: routing, no band keys.
	form := url.Values{"routing": {"randomised"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/objective/"+root.Slug+"/settings",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withObjectiveUser(req, user)
	req = withObjectiveSlug(req, root.Slug)
	h.ObjectiveSettingsPost(httptest.NewRecorder(), req)

	reloaded, err := h.objectiveService.GetByQuestIDAndSlug(ctx, user.CurrentQuestID, root.Slug)
	require.NoError(t, err)
	assert.Equal(t, "randomised", string(reloaded.Routing), "the routing changed")
	require.NotNil(t, reloaded.ChildrenMin, "and the band it never offered survived")
	assert.Equal(t, 1, *reloaded.ChildrenMin)
}

// The description reaches the database from the form, which the service tests
// cannot show: they call UpdateObjective directly, so a handler that never
// reads the field passes every one of them while the editor quietly discards
// what was typed.
func TestObjectiveEditPost_SavesTheDescription(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	ctx := context.Background()

	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)
	objective, err := h.objectiveService.CreateObjective(
		ctx, user.CurrentQuestID, root.ID, "Find the plan chest")
	require.NoError(t, err)

	post := func(form url.Values) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/admin/objective/"+objective.Slug,
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = withObjectiveUser(req, user)
		req = withObjectiveSlug(req, objective.Slug)
		h.ObjectiveEditPost(httptest.NewRecorder(), req)
	}
	reload := func() models.Objective {
		t.Helper()
		reloaded, err := h.objectiveService.GetByQuestIDAndSlug(
			ctx, user.CurrentQuestID, objective.Slug)
		require.NoError(t, err)
		return *reloaded
	}

	post(url.Values{
		"title":       {"Find the plan chest"},
		"description": {"Level 2, past the model-making room."},
	})
	assert.Equal(t, "Level 2, past the model-making room.", reload().Description)

	// Present and empty is an author clearing it, which has to save.
	post(url.Values{"title": {"Find the plan chest"}, "description": {""}})
	assert.Empty(t, reload().Description, "an empty field clears it")
}

// A form that does not offer the field leaves what another form stored alone:
// the settings popover posts routing and bands, and must not wipe a
// description written on the edit page.
func TestObjectiveSettingsPost_LeavesTheDescriptionAlone(t *testing.T) {
	dbc, cleanup := setupObjectiveTestDB(t)
	defer cleanup()
	h := newObjectiveTestHandler(t, dbc)
	user := objectiveTestQuest(t, dbc)
	ctx := context.Background()

	root, err := h.objectiveService.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)
	objective, err := h.objectiveService.CreateObjective(
		ctx, user.CurrentQuestID, root.ID, "Find the plan chest")
	require.NoError(t, err)
	require.NoError(t, h.objectiveService.UpdateObjective(ctx, &objective,
		services.ObjectiveUpdateData{Description: strPtr("Keep me.")}))

	form := url.Values{"routing": {"free_roam"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/objective/"+objective.Slug+"/settings",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withObjectiveUser(req, user)
	req = withObjectiveSlug(req, objective.Slug)
	h.ObjectiveSettingsPost(httptest.NewRecorder(), req)

	reloaded, err := h.objectiveService.GetByQuestIDAndSlug(ctx, user.CurrentQuestID, objective.Slug)
	require.NoError(t, err)
	assert.Equal(t, "Keep me.", reloaded.Description)
}
