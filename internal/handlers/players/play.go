package players

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
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
// Joining only attaches the session; StartGame (pressing the start button on
// /start) is what marks the run started, so a joining team always sees the
// start page's content first.
func (h *PlayerHandler) PlayWithCode(w http.ResponseWriter, r *http.Request) {
	runCode := chi.URLParam(r, "code")

	run, err := h.runService.GetRunByCode(r.Context(), runCode)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "PlayWithCode: looking up run", "error", err, "runCode", runCode)
		h.redirect(w, r, "/404")
		return
	}

	if err = h.startSession(w, r, runCode); err != nil {
		h.logger.ErrorContext(r.Context(), "PlayWithCode: starting session", "error", err, "runCode", runCode)
		h.redirect(w, r, "/404")
		return
	}

	h.redirect(w, r, landingPageFor(run))
}

// landingPageFor sends an already-started team straight back into the quest
// (a shared link revisited mid-game shouldn't re-show the start page), and a
// not-yet-started team to /start.
func landingPageFor(run *models.Run) string {
	if run.HasStarted {
		return "/objectives"
	}
	return "/start"
}

// PlayPost is the handler for the play form submission. Joining only
// attaches the session; StartGame (pressing the start button on /start) is
// what marks the run started, so a joining team always sees the start
// page's content first.
func (h *PlayerHandler) PlayPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "PlayPost: parsing form", "Error parsing form", "error", err)
		return
	}
	runCode := r.FormValue("run")

	run, err := h.runService.GetRunByCode(r.Context(), runCode)
	if err != nil {
		flashMsg := "Could not join this game. Please try again."
		if errors.Is(err, sql.ErrNoRows) {
			flashMsg = "Team not found: " + runCode
		}
		h.handleError(
			w,
			r,
			"PlayPost: looking up run",
			flashMsg,
			"error",
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

	h.redirect(w, r, landingPageFor(run))
}
