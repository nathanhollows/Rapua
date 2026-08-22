package players

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/flash"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
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

// PlayWithCode is a GET shortcut onto PlayPost's flow, for a run code shared as
// a link rather than typed into the form. It never creates a run.
func (h *PlayerHandler) PlayWithCode(w http.ResponseWriter, r *http.Request) {
	runCode := chi.URLParam(r, "code")

	err := h.runService.StartPlaying(r.Context(), runCode)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "PlayWithCode: starting game", "error", err, "runCode", runCode)
		h.redirect(w, r, "/404")
		return
	}

	if err = h.startSession(w, r, runCode); err != nil {
		h.logger.ErrorContext(r.Context(), "PlayWithCode: starting session", "error", err, "runCode", runCode)
		h.redirect(w, r, "/404")
		return
	}

	h.redirect(w, r, "/next")
}

// PlayPost is the handler for the play form submission.
func (h *PlayerHandler) PlayPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "PlayPost: parsing form", "Error parsing form", "error", err)
		return
	}
	runCode := r.FormValue("run")

	err = h.runService.StartPlaying(r.Context(), runCode)
	if err != nil {
		if errors.Is(err, services.ErrTeamNotFound) {
			h.handleError(
				w,
				r,
				"PlayPost: starting game",
				"Team not found: "+runCode,
				"Cannot start game with this team code",
				err,
				"teamCode",
				runCode,
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
			runCode,
		)
		return
	}

	err = h.startSession(w, r, runCode)
	if err != nil {
		h.handleError(
			w,
			r,
			"HomePost: starting session",
			"Error joining session. Please try again.",
			"error",
			err,
			"team",
			runCode,
		)
		return
	}

	h.redirect(w, r, "/next")
}
