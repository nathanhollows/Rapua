package handlers

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/players"
)

// Lobby is where teams wait for the game to begin.
func (h *PlayerHandler) Lobby(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	err = h.TeamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.Logger.Error("loading check ins", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	// If the user is in preview mode, only render the template, not the full layout.
	template := templates.Lobby(*team)
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		template = templates.Layout(template, "Lobby", team.Messages)
	}

	err = template.Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering lobby", "error", err.Error())
	}
}

// SetTeamName sets the team name.
func (h *PlayerHandler) SetTeamName(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	team.Name = r.FormValue("name")
	err = h.TeamService.Update(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "Error updating team", "Error updating team", "error", err)
		return
	}

	err = templates.TeamID(*team, true).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering team id", "error", err.Error())
	}
}
