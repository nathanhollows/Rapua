package middlewares

import (
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/models"
)

// StartMiddleware redirects to the start if the game is scheduled to start.
func StartMiddleware(_ teamService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Preview requests should pass through
		if r.Context().Value(contextkeys.PreviewKey) != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Check if team exists in context
		team := r.Context().Value(contextkeys.TeamKey)
		if team == nil {
			http.Redirect(w, r, "/play", http.StatusFound)
			return
		}

		// Type assertion
		foundTeam, ok := team.(*models.Team)
		if !ok || foundTeam == nil || foundTeam.Code == "" {
			http.Redirect(w, r, "/play", http.StatusFound)
			return
		}

		// Redirect to start if game is scheduled
		// Exception: allow block state endpoints needed for start page functionality
		isBlockStateEndpoint := strings.HasPrefix(r.URL.Path, "/blocks/") &&
			(strings.HasSuffix(r.URL.Path, "/team-name-block") ||
				strings.HasSuffix(r.URL.Path, "/game-status-alert") ||
				strings.HasSuffix(r.URL.Path, "/start-game-button"))

		if foundTeam.Instance.GetStatus() != models.Active &&
			!strings.HasPrefix(r.URL.Path, "/start") &&
			!isBlockStateEndpoint {
			http.Redirect(w, r, "/start", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
