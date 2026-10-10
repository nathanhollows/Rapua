package players

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

// Team is /team's handler: who the run is playing as, and where they sit if the
// quest keeps a board.
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

	page := templates.Team(templates.TeamParams{
		Run:         *team,
		Leaderboard: h.leaderboard(r, team),
		Objectives:  len(team.Quest.Objectives),
	})
	if err := templates.AppLayout(
		page, templates.TeamChrome(*team), "Team", team.Messages,
	).Render(r.Context(), w); err != nil {
		h.handleError(w, r, "Team: rendering template", "Error rendering template", "error", err)
	}
}
