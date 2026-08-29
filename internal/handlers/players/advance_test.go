package players_test

import (
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/stretchr/testify/assert"
)

// Regression test: AdvanceGroup used to always validate through the
// location-only GetPlayerNavigationView. On an objective-built quest every
// group has empty LocationIDs, so the walk trivially "completed" and the
// service returned ErrAllLocationsVisited: the error toast was silently
// discarded by hx-swap=none, and the non-htmx fallback redirected to /next,
// which itself immediately bounces to /complete. The Skip button was a
// silent no-op. It must now redirect to /objectives instead.
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
