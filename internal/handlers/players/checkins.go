package players

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v6/internal/flash"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
	"github.com/nathanhollows/Rapua/v6/models"
)

// CheckIn handles the GET request for scanning a location.
func (h *PlayerHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	code = strings.ToUpper(code)

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		team = &models.Team{}
	}

	if team.MustCheckOut != "" {
		err := h.teamService.LoadRelation(r.Context(), team, "BlockingLocation")
		if err != nil {
			h.logger.Error("CheckIn: loading blocking location", "err", err)
			http.Redirect(w, r, r.Header.Get("/next"), http.StatusFound)
			return
		}
	}

	marker, err := h.markerService.GetMarkerByCode(r.Context(), code)
	if err != nil {
		h.logger.Error("CheckOut: getting marker by code", "error", err.Error())
		h.redirect(w, r, "/404")
		return
	}

	c := templates.CheckIn(marker, team.Code, team.BlockingLocation)
	err = templates.Layout(c, "Check Out: "+marker.Name, team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering checkin", "error", err.Error())
	}
}

// CheckInPost handles the POST request for scanning in.
func (h *PlayerHandler) CheckInPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}
	locationCode := chi.URLParam(r, "code")
	locationCode = strings.ToUpper(locationCode)

	// Get the team from the context
	// Or start a new session if the provided team code is valid
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		team, err = h.teamService.GetTeamByCode(r.Context(), r.FormValue("team"))
		if err != nil {
			h.handleError(
				w,
				r,
				"CheckInPost: getting team by code",
				"Error finding team. Please double check your team code.",
				"error",
				err,
				"team",
				r.FormValue("team"),
			)
			return
		}
		err = h.startSession(w, r, team.Code)
		if err != nil {
			h.handleError(
				w,
				r,
				"CheckInPost: starting session",
				"Error starting session. Please try again.",
				"error",
				err,
				"team",
				team.Code,
			)
			return
		}
	}

	err = h.checkInService.CheckIn(r.Context(), team, locationCode)
	if err != nil {
		if errors.Is(err, services.ErrLocationNotFound) {
			h.handleError(
				w,
				r,
				"CheckInPost: checking in",
				"Location not found. Please try again.",
				"error",
				err,
				"team",
				team.Code,
				"location",
				locationCode,
			)
			return
		}
		if errors.Is(err, services.ErrAlreadyCheckedIn) {
			h.handleError(
				w,
				r,
				"CheckInPost: checking in",
				"You have already checked in here.",
				"error",
				err,
				"team",
				team.Code,
				"location",
				locationCode,
			)
			return
		}
		h.handleError(
			w,
			r,
			"CheckInPost: checking in",
			"Error checking in",
			"error",
			err,
			"team",
			team.Code,
			"location",
			locationCode,
		)
		return
	}

	h.redirect(w, r, "/checkins/"+locationCode)
}

func (h *PlayerHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	code = strings.ToUpper(code)

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		team = &models.Team{}
	}

	if team.MustCheckOut != "" {
		err := h.teamService.LoadRelation(r.Context(), team, "BlockingLocation")
		if err != nil {
			h.logger.Error("CheckIn: loading blocking location", "err", err)
			// TODO: render error page
			h.redirect(w, r, "/404")
			return
		}
	}

	marker, err := h.markerService.GetMarkerByCode(r.Context(), code)
	if err != nil {
		h.logger.Error("CheckOut: getting marker by code", "error", err.Error())
		h.redirect(w, r, "/404")
		return
	}

	c := templates.CheckOut(marker, team.Code, team.BlockingLocation)
	err = templates.Layout(c, "Check Out: "+marker.Name, team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering checkin", "error", err.Error())
	}
}

func (h *PlayerHandler) CheckOutPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}
	locationCode := chi.URLParam(r, "code")
	locationCode = strings.ToUpper(locationCode)

	// Get the team from the context
	// Or start a new session if the provided team code is valid
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		team, err = h.teamService.GetTeamByCode(r.Context(), r.FormValue("team"))
		if err != nil {
			h.handleError(
				w,
				r,
				"CheckInPost: getting team by code",
				"Error finding team. Please double check your team code.",
				"error",
				err,
				"team",
				r.FormValue("team"),
			)
			return
		}
		err = h.startSession(w, r, team.Code)
		if err != nil {
			h.handleError(
				w,
				r,
				"CheckInPost: starting session",
				"Error starting session. Please try again.",
				"error",
				err,
				"team",
				team.Code,
			)
			return
		}
	}

	err = h.checkInService.CheckOut(r.Context(), team, locationCode)

	var message *flash.Message
	if err != nil {
		switch {
		case errors.Is(err, services.ErrLocationNotFound):
			message = flash.NewError("Location not found. Please double check the code and try again.")
		case errors.Is(err, services.ErrUnecessaryCheckOut):
			message = flash.NewInfo("You are not checked in anywhere.")
		case errors.Is(err, services.ErrCheckOutAtWrongLocation):
			message = flash.NewInfo("You are not checked in here.")
		case errors.Is(err, services.ErrUnfinishedCheckIn):
			message = flash.NewWarning("Try completing all activities first!")
		default:
			message = flash.NewError("Error checking out. Please try again.")
			h.logger.Error("Check Out post", "err", err.Error())
		}
		err = templates.Toast(*message).Render(r.Context(), w)
		if err != nil {
			h.handleError(
				w,
				r,
				"CheckOutPost: checking out",
				"Error checking out",
				"error",
				err,
				"team",
				team.Code,
				"location",
				locationCode,
			)
		}
		return
	}

	h.redirect(w, r, "/next")
}

// MyCheckins shows the found locations page.
func (h *PlayerHandler) MyCheckins(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil || team == nil {
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	err = h.teamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.Error("loading check ins", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	c := templates.MyCheckins(*team)
	err = templates.Layout(c, "My Check-ins", team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering checkins", "error", err.Error())
	}
}
