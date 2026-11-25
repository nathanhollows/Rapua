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

	rejoined := h.sessionHoldsRun(r, run.Code)
	if err = h.startSession(w, r, run.Code); err != nil {
		h.logger.ErrorContext(r.Context(), "PlayWithCode: starting session", "error", err, "runCode", runCode)
		h.redirect(w, r, "/404")
		return
	}
	if !rejoined {
		h.recordJoined(r, run, models.JoinViaLink)
	}

	h.redirect(w, r, landingPageFor(run))
}

// recordJoined notes a join in the run's stream. A failure is logged and
// ignored, so a missing row never turns a player away.
func (h *PlayerHandler) recordJoined(r *http.Request, run *models.Run, via string) {
	if err := h.runService.RecordJoined(r.Context(), run, via); err != nil {
		h.logger.ErrorContext(r.Context(), "recording join", "error", err, "team", run.Code)
	}
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

// PlayCheck validates a code without joining, so a mistyped code that matches
// another team cannot move a returning player onto it.
func (h *PlayerHandler) PlayCheck(w http.ResponseWriter, r *http.Request) {
	_, err := h.runService.GetRunByCode(r.Context(), r.FormValue("run"))
	w.Header().Set("Content-Type", "application/json")
	switch {
	case err == nil:
		_, _ = w.Write([]byte(`{"found":true}`))
	case errors.Is(err, sql.ErrNoRows):
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"found":false}`))
	default:
		h.logger.ErrorContext(r.Context(), "PlayCheck: looking up run", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"found":false}`))
	}
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

	rejoined := h.sessionHoldsRun(r, run.Code)
	err = h.startSession(w, r, run.Code)
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

	if !rejoined {
		h.recordJoined(r, run, models.JoinViaForm)
	}
	h.redirect(w, r, landingPageFor(run))
}
