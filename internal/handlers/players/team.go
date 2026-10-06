package players

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

// Team is /team's handler: who the run is playing as.
//
// No leaderboard yet. The board needs the leaderboard service, which this
// handler does not hold, and the page stands up without it: a name and the code
// someone else joins with is the part a player comes here for.
func (h *PlayerHandler) Team(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if err := h.runService.LoadRelations(r.Context(), team); err != nil {
		h.handleError(w, r, "Team: loading relations", "Error loading your team", "error", err)
		return
	}

	page := templates.Team(templates.TeamParams{Run: *team})
	if err := templates.AppLayout(
		page, templates.TeamChrome(*team), "Team", team.Messages,
	).Render(r.Context(), w); err != nil {
		h.handleError(w, r, "Team: rendering template", "Error rendering template", "error", err)
	}
}
