package players_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/schema"
)

// Regression test: StartGame is the only place a run gets marked started.
// Before this fix, PlayPost/PlayWithCode marked the run started (and
// deducted a credit) the moment a code was entered, letting players skip
// the start page's content entirely and land straight on /objectives.
func TestPlayerHandler_StartGame_Success(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()

	instance, _, team := createTestObjectiveQuest(t, dbc, instanceRepo)
	instance.StartTime = schema.NullTime{Time: time.Now().Add(-1 * time.Hour)}
	require.NoError(t, instanceRepo.Update(context.Background(), instance))
	team.Quest = *instance

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	require.False(t, team.HasStarted, "sanity check: joining must not have started the run already")

	req := requestWithRun("/start", team)
	w := httptest.NewRecorder()
	handler.StartGame(w, req)

	assert.Equal(t, "/objectives", w.Header().Get("Location"))

	reloaded, err := runService.GetRunByCode(context.Background(), team.Code)
	require.NoError(t, err)
	assert.True(t, reloaded.HasStarted, "pressing Start must mark the run started")
	assert.False(t, reloaded.StartedAt.IsZero(), "pressing Start must stamp StartedAt")
}

// A team can't start a game that isn't open yet: the button only renders as
// clickable when active, but the server must not trust that alone.
func TestPlayerHandler_StartGame_QuestNotActive(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()

	// createTestObjectiveQuest leaves StartTime zero, i.e. Closed.
	_, _, team := createTestObjectiveQuest(t, dbc, instanceRepo)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	req := requestWithRun("/start", team)
	w := httptest.NewRecorder()
	handler.StartGame(w, req)

	assert.NotEqual(t, "/objectives", w.Header().Get("Location"))

	reloaded, err := runService.GetRunByCode(context.Background(), team.Code)
	require.NoError(t, err)
	assert.False(t, reloaded.HasStarted, "a closed/scheduled game must not be startable")
}

// A run whose owner has no credit left must fail gracefully (a toast, not a
// redirect into the game) and must not be marked started.
func TestPlayerHandler_StartGame_InsufficientCredits(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()

	instance, _, team := createTestObjectiveQuest(t, dbc, instanceRepo)
	instance.StartTime = schema.NullTime{Time: time.Now().Add(-1 * time.Hour)}
	require.NoError(t, instanceRepo.Update(context.Background(), instance))
	team.Quest = *instance
	zeroCredits(t, dbc, instance.UserID)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	req := requestWithRun("/start", team)
	w := httptest.NewRecorder()
	handler.StartGame(w, req)

	assert.NotEqual(t, "/objectives", w.Header().Get("Location"))

	reloaded, err := runService.GetRunByCode(context.Background(), team.Code)
	require.NoError(t, err)
	assert.False(t, reloaded.HasStarted, "insufficient credits must not mark the run started")
}
