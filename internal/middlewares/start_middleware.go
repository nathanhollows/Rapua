package middlewares

import (
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/models"
)

// StartMiddleware redirects to /start whenever the game isn't open yet, or
// the team hasn't pressed the start button yet: PlayPost/PlayWithCode create
// the run's session without marking it started, so this is what actually
// keeps a team on the start page until they act on it.
func StartMiddleware(_ runService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Preview requests should pass through
		if r.Context().Value(contextkeys.PreviewKey) != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Check if team exists in context
		team := r.Context().Value(contextkeys.RunKey)
		if team == nil {
			http.Redirect(w, r, "/play", http.StatusFound)
			return
		}

		// Type assertion
		foundTeam, ok := team.(*models.Run)
		if !ok || foundTeam == nil || foundTeam.Code == "" {
			http.Redirect(w, r, "/play", http.StatusFound)
			return
		}

		// Redirect to start if the game isn't open yet, or the team hasn't
		// started (pressed the start button) yet.
		// Exception: allow block state endpoints needed for start page functionality
		isBlockStateEndpoint := strings.HasPrefix(r.URL.Path, "/blocks/") &&
			(strings.HasSuffix(r.URL.Path, "/team-name-block") ||
				strings.HasSuffix(r.URL.Path, "/game-status-alert") ||
				strings.HasSuffix(r.URL.Path, "/start-game-button"))

		if (foundTeam.Quest.GetStatus() != models.Active || !foundTeam.HasStarted) &&
			!strings.HasPrefix(r.URL.Path, "/start") &&
			!isBlockStateEndpoint {
			http.Redirect(w, r, "/start", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
