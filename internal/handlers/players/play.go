package players

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/players"
	"github.com/nathanhollows/Rapua/v4/models"
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
		h.logger.Error("Home: rendering template", "error", err)
	}
}

// PlayPost is the handler for the play form submission.
func (h *PlayerHandler) PlayPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "PlayPost: parsing form", "Error parsing form", "error", err)
		return
	}
	teamCode := r.FormValue("team")

	err = h.teamService.StartPlaying(r.Context(), teamCode)
	if err != nil {
		if err == services.ErrTeamNotFound {
			h.handleError(w, r, "PlayPost: starting game", "Team not found: "+teamCode, "Cannot start game with this team code", err, "teamCode", teamCode)
			return
		}
		h.handleError(w, r, "PlayPost: starting game", "Error joining game", "Could not start game", err, "teamCode", teamCode)
		return
	}

	err = h.startSession(w, r, teamCode)
	if err != nil {
		h.handleError(w, r, "HomePost: starting session", "Error joining session. Please try again.", "error", err, "team", teamCode)
		return
	}

	h.redirect(w, r, "/next")
}
