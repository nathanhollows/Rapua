package players_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Setenv("SESSION_KEY", "test-session-key-for-players-handler!")
	sessions.Start()
	os.Exit(m.Run())
}

// requestWithCode fakes the chi param httptest doesn't populate on its own.
func requestWithCode(code string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/play/"+code, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", code)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestPlayerHandler_PlayWithCode_Success(t *testing.T) {
	runService, dbc, cleanup := setupRunService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	code := createTestRun(t, dbc, parents.QuestID)

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))
	w := httptest.NewRecorder()
	handler.PlayWithCode(w, requestWithCode(code))

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/objectives", w.Header().Get("Location"))
	require.NotEmpty(t, w.Result().Cookies(), "starting the session should set a cookie")

	run, err := runService.GetRunByCode(context.Background(), code)
	require.NoError(t, err)
	assert.True(t, run.HasStarted)
}

func TestPlayerHandler_PlayWithCode_UnknownCode(t *testing.T) {
	runService, _, cleanup := setupRunService(t)
	defer cleanup()

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))
	w := httptest.NewRecorder()
	handler.PlayWithCode(w, requestWithCode("NOPE1"))

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/404", w.Header().Get("Location"))
	assert.Empty(t, w.Result().Cookies(), "a run that never started should not get a session")
}

// A run that exists but whose owner has no credit left must fail the same way
// as one that never existed: no session, no /next.
func TestPlayerHandler_PlayWithCode_InsufficientCredits(t *testing.T) {
	runService, dbc, cleanup := setupRunService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	code := createTestRun(t, dbc, parents.QuestID)
	zeroCredits(t, dbc, parents.UserID)

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))
	w := httptest.NewRecorder()
	handler.PlayWithCode(w, requestWithCode(code))

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/404", w.Header().Get("Location"))
	assert.Empty(t, w.Result().Cookies(), "a run that failed to start should not get a session")
}

// Visiting the same link twice is the common case for a shared link: the
// second visit must not error just because the run already started.
func TestPlayerHandler_PlayWithCode_AlreadyStarted(t *testing.T) {
	runService, dbc, cleanup := setupRunService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	code := createTestRun(t, dbc, parents.QuestID)

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))

	first := httptest.NewRecorder()
	handler.PlayWithCode(first, requestWithCode(code))
	require.Equal(t, "/objectives", first.Header().Get("Location"))

	second := httptest.NewRecorder()
	handler.PlayWithCode(second, requestWithCode(code))
	assert.Equal(t, "/objectives", second.Header().Get("Location"))
}
