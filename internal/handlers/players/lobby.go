package handlers

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/flash"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/players"
)

// Lobby is where teams wait for the game to begin.
func (h *PlayerHandler) Lobby(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		flash.NewError("Error loading team.").Save(w, r)
		h.redirect(w, r, "/play")
		return
	}

	c := templates.Lobby(*team)
	err = templates.Layout(c, "Lobby", team.Messages).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering lobby", "error", err.Error())
	}
}

// SetTeamName sets the team name.
func (h *PlayerHandler) SetTeamName(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		flash.NewError("Error loading team.").Save(w, r)
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
