package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/flash"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v7/models"
)

// StartGame starts the game immediately.
func (h *Handler) StartGame(w http.ResponseWriter, r *http.Request) {
	h.toggleGameStatus(w, r, "starting", "Game started!", h.gameScheduleService.Start)
}

// StopGame stops the game immediately.
func (h *Handler) StopGame(w http.ResponseWriter, r *http.Request) {
	h.toggleGameStatus(w, r, "stopping", "Game stopped!", h.gameScheduleService.Stop)
}

// toggleGameStatus is a helper for StartGame and StopGame that avoids duplicating
// the context lookup, error handling, flash message, and template rendering.
func (h *Handler) toggleGameStatus(
	w http.ResponseWriter,
	r *http.Request,
	action string,
	successMsg string,
	serviceFn func(context.Context, *models.Quest) error,
) {
	user := h.UserFromContext(r.Context())

	err := serviceFn(r.Context(), &user.CurrentQuest)
	if err != nil {
		h.handleError(
			w,
			r,
			action+" game",
			"Error "+action+" game",
			"Could not "+action+" game",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	msg := *flash.NewSuccess(successMsg)
	err = templates.GameScheduleStatus(user.CurrentQuest, msg).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), action+" game: rendering GameScheduleStatus template", "error", err)
		return
	}
	err = templates.ActivityStatusBadgeOOB(user.CurrentQuest).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), action+" game: rendering ActivityStatusBadgeOOB template", "error", err)
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
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	err = h.gameScheduleService.ScheduleGame(r.Context(), &user.CurrentQuest, sTime, eTime)
	if err != nil {
		h.handleError(
			w,
			r,
			"ScheduleGame: scheduling game",
			"Error scheduling game",
			"Could not schedule game",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	err = templates.GameScheduleStatus(user.CurrentQuest, *flash.NewSuccess("Schedule updated!")).
		Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ScheduleGame: rendering GameScheduleStatus template", "error", err)
		return
	}
	err = templates.ActivityStatusBadgeOOB(user.CurrentQuest).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ScheduleGame: rendering ActivityStatusBadgeOOB template", "error", err)
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
			"quest_id",
			user.CurrentQuestID,
		)
		return time.Time{}, false
	}

	parsedTime, err := parseDateTime(date, timeValue)
	if err != nil {
		h.handleError(
			w,
			r,
			"ScheduleGame: parsing "+timeType+" date and time",
			"Error parsing "+timeType+" date and time",
			"Could not parse date and time",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return time.Time{}, false
	}

	return parsedTime, true
}

func parseDateTime(dateString, timeString string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", dateString)
	if err != nil {
		return time.Time{}, err
	}

	const (
		shortTimeLen   = 2
		mediumTimeLen  = 5
		secondsSuffix  = ":00"
		fullTimeSuffix = ":00:00"
	)
	switch len(timeString) {
	case mediumTimeLen:
		timeString += secondsSuffix
	case shortTimeLen:
		timeString += fullTimeSuffix
	}
	t, err := time.Parse("15:04:05", timeString)
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(
		d.Year(),
		d.Month(),
		d.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		t.Nanosecond(),
		t.UTC().Location(),
	), nil
}
