package players

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

func (h *PlayerHandler) You(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if err := h.runService.LoadRelations(r.Context(), team); err != nil {
		h.handleError(w, r, "You: loading relations", "Error loading your team", "error", err)
		return
	}

	page := templates.You(templates.YouParams{
		Run:         *team,
		Leaderboard: h.leaderboard(r, team),
	})
	if err := templates.AppLayout(
		page, templates.YouChrome(*team), "You", team.Messages,
	).Render(r.Context(), w); err != nil {
		h.handleError(w, r, "You: rendering template", "Error rendering template", "error", err)
	}
}
