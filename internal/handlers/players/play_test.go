package players_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postPlayForm(code string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/play", strings.NewReader(url.Values{"run": {code}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

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
	assert.Equal(t, "/start", w.Header().Get("Location"), "joining sends an unstarted team to /start, not straight in")
	require.NotEmpty(t, w.Result().Cookies(), "starting the session should set a cookie")

	run, err := runService.GetRunByCode(context.Background(), code)
	require.NoError(t, err)
	assert.False(t, run.HasStarted, "joining must not mark the run started; only pressing Start does")
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

// Credits are no longer checked at join time: StartPlaying (and its credit
// deduction) only runs when the team presses Start, so a run whose owner has
// no credit left still joins successfully and lands on /start. See
// TestPlayerHandler_StartGame_InsufficientCredits for the actual gate.
func TestPlayerHandler_PlayWithCode_JoinsRegardlessOfCredits(t *testing.T) {
	runService, dbc, cleanup := setupRunService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	code := createTestRun(t, dbc, parents.QuestID)
	zeroCredits(t, dbc, parents.UserID)

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))
	w := httptest.NewRecorder()
	handler.PlayWithCode(w, requestWithCode(code))

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/start", w.Header().Get("Location"))
	assert.NotEmpty(t, w.Result().Cookies(), "joining should get a session regardless of credits")
}

// Visiting the same link twice is the common case for a shared link: the
// first visit (not yet started) goes to /start; once the team has actually
// started, a later visit skips straight back into the game.
func TestPlayerHandler_PlayWithCode_AlreadyStarted(t *testing.T) {
	runService, dbc, cleanup := setupRunService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	code := createTestRun(t, dbc, parents.QuestID)

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))

	first := httptest.NewRecorder()
	handler.PlayWithCode(first, requestWithCode(code))
	require.Equal(t, "/start", first.Header().Get("Location"))

	require.NoError(t, runService.StartPlaying(context.Background(), code))

	second := httptest.NewRecorder()
	handler.PlayWithCode(second, requestWithCode(code))
	assert.Equal(t, "/objectives", second.Header().Get("Location"))
}

// An unknown code is a real "check your code" mistake, so the flash message
// should say so.
func TestPlayerHandler_PlayPost_UnknownCode(t *testing.T) {
	runService, _, cleanup := setupRunService(t)
	defer cleanup()

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))
	w := httptest.NewRecorder()
	handler.PlayPost(w, postPlayForm("NOPE1"))

	assert.Contains(t, w.Body.String(), "Team not found")
}

// Regression test: a lookup failure that isn't "no such code" (e.g. a DB
// outage) must not be mislabelled as a bad team code. PlayPost used to call
// GetRunByCode directly and blame the code for any error it returned.
func TestPlayerHandler_PlayPost_LookupErrorIsNotMislabelledAsBadCode(t *testing.T) {
	runService, dbc, cleanup := setupRunService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	code := createTestRun(t, dbc, parents.QuestID)

	require.NoError(t, dbc.Close(), "force a real lookup error, not sql.ErrNoRows")

	handler := players.NewTestPlayerHandler(players.WithRunService(runService))
	w := httptest.NewRecorder()
	handler.PlayPost(w, postPlayForm(code))

	assert.NotContains(t, w.Body.String(), "Team not found",
		"a DB-level failure must not be presented as a bad team code")
	assert.Contains(t, w.Body.String(), "Could not join this game")
}
