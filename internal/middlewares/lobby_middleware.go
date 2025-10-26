package middlewares

import (
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v5/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v5/models"
)

// LobbyMiddleware redirects to the lobby if the game is scheduled to start.
func LobbyMiddleware(teamService teamService, next http.Handler) http.Handler {
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

		// Redirect to lobby if game is scheduled
		if foundTeam.Instance.GetStatus() != models.Active &&
			!strings.HasPrefix(r.URL.Path, "/lobby") &&
			!strings.HasPrefix(r.URL.Path, "lobby") {
			http.Redirect(w, r, "/lobby", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
