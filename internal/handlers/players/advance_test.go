package players_test

import (
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/stretchr/testify/assert"
)

// Regression test: the Skip button must produce an hx redirect to /objectives
// rather than an error path, which hx-swap=none silently discards and so made
// the button look like a no-op.
func TestPlayerHandler_AdvanceGroup_ObjectiveQuest_RedirectsToObjectives(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()

	_, team := createTestObjectiveQuest(t, dbc, instanceRepo)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	req := requestWithRun("/advance", team)
	req.Header.Set("Hx-Request", "true")
	w := httptest.NewRecorder()
	handler.AdvanceGroup(w, req)

	assert.Equal(t, "/objectives", w.Header().Get("Hx-Redirect"))
}
