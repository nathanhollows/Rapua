package admin

import (
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v4/helpers"
	"github.com/nathanhollows/Rapua/v4/internal/flash"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
)

// StartGame starts the game immediately.
func (h *AdminHandler) StartGame(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.gameScheduleService.Start(r.Context(), &user.CurrentInstance)
	if err != nil {
		h.handleError(w, r, "starting game", "Error starting game", "Could not start game", err, "instance_id", user.CurrentInstanceID)
		return
	}

	msg := *flash.NewSuccess("Game started!")
	err = templates.GameScheduleStatus(user.CurrentInstance, msg).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("StartGame: rendering template", "error", err)
	}
}

// StopGame stops the game immediately.
func (h *AdminHandler) StopGame(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.gameScheduleService.Stop(r.Context(), &user.CurrentInstance)
	if err != nil {
		h.handleError(w, r, "stopping game", "Error stopping game", "Could not stop game", err, "instance_id", user.CurrentInstanceID)
		return
	}

	msg := *flash.NewSuccess("Game stopped!")
	err = templates.GameScheduleStatus(user.CurrentInstance, msg).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("StopGame: rendering template", "error", err)
	}
}

// ScheduleGame schedules the game to start and/or end at a specific time.
func (h *AdminHandler) ScheduleGame(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	var sTime, eTime time.Time
	if r.Form.Get("set_start") != "" {
		startDate := r.Form.Get("utc_start_date")
		startTime := r.Form.Get("utc_start_time")
		if startDate == "" || startTime == "" {
			h.handleError(w, r, "ScheduleGame: missing start date or time", "Error parsing start date and time", "Start date and time are required", nil, "instance_id", user.CurrentInstanceID)
			return
		}
		sTime, err = helpers.ParseDateTime(startDate, startTime)
		if err != nil {
			h.handleError(w, r, "ScheduleGame: parsing start date and time", "Error parsing start date and time", "Could not parse date and time", err, "instance_id", user.CurrentInstanceID)
			return
		}
	}

	if r.Form.Get("set_end") != "" {
		endDate := r.Form.Get("utc_end_date")
		endTime := r.Form.Get("utc_end_time")
		if endDate == "" || endTime == "" {
			h.handleError(w, r, "ScheduleGame: missing end date or time", "Error parsing end date and time", "End date and time are required", nil, "instance_id", user.CurrentInstanceID)
			return
		}
		eTime, err = helpers.ParseDateTime(endDate, endTime)
		if err != nil {
			h.handleError(w, r, "ScheduleGame: parsing end date and time", "Error parsing end date and time", "Could not parse date and time", err, "instance_id", user.CurrentInstanceID)
			return
		}
	}

	if sTime.After(eTime) && !eTime.IsZero() {
		h.handleError(w, r, "ScheduleGame: start time after end time", "Error scheduling game", "Start time must be before end time", nil, "instance_id", user.CurrentInstanceID)
		return
	}

	err = h.gameScheduleService.ScheduleGame(r.Context(), &user.CurrentInstance, sTime, eTime)
	if err != nil {
		h.handleError(w, r, "ScheduleGame: scheduling game", "Error scheduling game", "Could not schedule game", err, "instance_id", user.CurrentInstanceID)
		return
	}

	err = templates.GameScheduleStatus(user.CurrentInstance, *flash.NewSuccess("Schedule updated!")).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("ScheduleGame: rendering template", "error", err)
	}
}
