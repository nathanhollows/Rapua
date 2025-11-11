package admin

import (
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v6/helpers"
	"github.com/nathanhollows/Rapua/v6/internal/flash"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v6/models"
)

// StartGame starts the game immediately.
func (h *Handler) StartGame(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.gameScheduleService.Start(r.Context(), &user.CurrentInstance)
	if err != nil {
		h.handleError(
			w,
			r,
			"starting game",
			"Error starting game",
			"Could not start game",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	msg := *flash.NewSuccess("Game started!")
	err = templates.GameScheduleStatus(user.CurrentInstance, msg).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("StartGame: rendering template", "error", err)
	}
}

// StopGame stops the game immediately.
func (h *Handler) StopGame(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.gameScheduleService.Stop(r.Context(), &user.CurrentInstance)
	if err != nil {
		h.handleError(
			w,
			r,
			"stopping game",
			"Error stopping game",
			"Could not stop game",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	msg := *flash.NewSuccess("Game stopped!")
	err = templates.GameScheduleStatus(user.CurrentInstance, msg).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("StopGame: rendering template", "error", err)
	}
}

// ScheduleGame schedules the game to start and/or end at specified times.
func (h *Handler) ScheduleGame(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	sTime, ok := h.parseScheduleTime(w, r, user, "start", "set_start", "utc_start_date", "utc_start_time")
	if !ok {
		return
	}

	eTime, ok := h.parseScheduleTime(w, r, user, "end", "set_end", "utc_end_date", "utc_end_time")
	if !ok {
		return
	}

	if sTime.After(eTime) && !eTime.IsZero() {
		h.handleError(
			w,
			r,
			"ScheduleGame: start time after end time",
			"Error scheduling game",
			"Start time must be before end time",
			nil,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = h.gameScheduleService.ScheduleGame(r.Context(), &user.CurrentInstance, sTime, eTime)
	if err != nil {
		h.handleError(
			w,
			r,
			"ScheduleGame: scheduling game",
			"Error scheduling game",
			"Could not schedule game",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = templates.GameScheduleStatus(user.CurrentInstance, *flash.NewSuccess("Schedule updated!")).
		Render(r.Context(), w)
	if err != nil {
		h.logger.Error("ScheduleGame: rendering template", "error", err)
	}
}

// parseScheduleTime parses and validates a schedule time from form values.
func (h *Handler) parseScheduleTime(
	w http.ResponseWriter,
	r *http.Request,
	user *models.User,
	timeType string,
	setFlag string,
	dateParam string,
	timeParam string,
) (time.Time, bool) {
	if r.Form.Get(setFlag) == "" {
		return time.Time{}, true
	}

	date := r.Form.Get(dateParam)
	timeValue := r.Form.Get(timeParam)

	if date == "" || timeValue == "" {
		h.handleError(
			w,
			r,
			"ScheduleGame: missing "+timeType+" date or time",
			"Error parsing "+timeType+" date and time",
			timeType+" date and time are required",
			nil,
			"instance_id",
			user.CurrentInstanceID,
		)
		return time.Time{}, false
	}

	parsedTime, err := helpers.ParseDateTime(date, timeValue)
	if err != nil {
		h.handleError(
			w,
			r,
			"ScheduleGame: parsing "+timeType+" date and time",
			"Error parsing "+timeType+" date and time",
			"Could not parse date and time",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return time.Time{}, false
	}

	return parsedTime, true
}
