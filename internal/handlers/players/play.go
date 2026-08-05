package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v7/internal/flash"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/players"
	"github.com/nathanhollows/Rapua/v7/models"
)

// Play shows the player the first page of the game.
func (h *PlayerHandler) Play(w http.ResponseWriter, r *http.Request) {
	team, _ := h.getRunFromContext(r.Context())

	if team == nil {
		team = &models.Run{}
	}

	c := templates.Home(*team)
	err := templates.Layout(c, "Home", nil).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Home: rendering template", "error", err)
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

	err = h.runService.StartPlaying(r.Context(), teamCode)
	if err != nil {
		if errors.Is(err, services.ErrTeamNotFound) {
			h.handleError(
				w,
				r,
				"PlayPost: starting game",
				"Team not found: "+teamCode,
				"Cannot start game with this team code",
				err,
				"teamCode",
				teamCode,
			)
			return
		}
		if errors.Is(err, services.ErrInsufficientCredits) {
			err = templates.Toast(*flash.NewError("Unable to start game. The host has been notified.")).
				Render(r.Context(), w)
			if err != nil {
				h.logger.ErrorContext(r.Context(), "rendering template", "error", err)
			}
			return
		}
		h.handleError(
			w,
			r,
			"PlayPost: starting game",
			"Error joining game",
			"Could not start game",
			err,
			"teamCode",
			teamCode,
		)
		return
	}

	err = h.startSession(w, r, teamCode)
	if err != nil {
		h.handleError(
			w,
			r,
			"HomePost: starting session",
			"Error joining session. Please try again.",
			"error",
			err,
			"team",
			teamCode,
		)
		return
	}

	h.redirect(w, r, "/next")
}
