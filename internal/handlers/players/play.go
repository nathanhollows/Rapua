package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/players"
	"github.com/nathanhollows/Rapua/v3/models"
)

// Play shows the player the first page of the game.
func (h *PlayerHandler) Play(w http.ResponseWriter, r *http.Request) {
	team, _ := h.getTeamFromContext(r.Context())

	if team == nil {
		team = &models.Team{}
	}

	c := templates.Home(*team)
	err := templates.Layout(c, "Home", nil).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Home: rendering template", "error", err)
	}
}

// PlayPost is the handler for the play form submission.
func (h *PlayerHandler) PlayPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}
	teamCode := r.FormValue("team")
	teamName := r.FormValue("customTeamName")

	response := h.GameplayService.StartPlaying(r.Context(), teamCode, teamName)
	if response.Error != nil {
		err := templates.Toast(response.FlashMessages...).Render(r.Context(), w)
		if err != nil {
			h.Logger.Error("HomePost: rendering template", "error", err)
			return
		}
		return
	}

	team := response.Data["team"].(*models.Team)

	err = h.startSession(w, r, team.Code)
	if err != nil {
		h.handleError(w, r, "HomePost: starting session", "Error starting session. Please try again.", "error", err, "team", team.Code)
		return
	}

	h.redirect(w, r, "/next")
}
