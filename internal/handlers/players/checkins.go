package players

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/flash"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
)

func (h *PlayerHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))

	marker, err := h.markerService.GetMarkerByCode(r.Context(), code)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "CheckOut: getting marker by code", "error", err.Error())
		h.redirect(w, r, "/404")
		return
	}

	team, err := h.getRunFromContext(r.Context())
	if err != nil || team == nil {
		h.renderCheckOutForm(w, r, marker, &models.Run{})
		return
	}

	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.renderCheckOutForm(w, r, marker, team)
		return
	}

	if team.MustCheckOut != "" {
		_ = h.runService.LoadBlockingLocation(r.Context(), team)
	}

	if team.MustCheckOut == "" {
		h.renderCheckOutForm(w, r, marker, team)
		return
	}

	location, err := h.locationService.GetByID(r.Context(), team.MustCheckOut)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "CheckOut: getting location", "err", err)
		h.renderCheckOutForm(w, r, marker, team)
		return
	}

	if location.MarkerID != code {
		h.renderCheckOutForm(w, r, marker, team)
		return
	}

	err = h.checkInService.CheckOut(r.Context(), team, code)
	if err != nil {
		if !errors.Is(err, services.ErrUnfinishedCheckIn) {
			h.logger.ErrorContext(
				r.Context(),
				"CheckOut: auto check-out failed",
				"error",
				err.Error(),
				"run",
				team.Code,
				"location",
				code,
			)
		}
		h.renderCheckOutForm(w, r, marker, team)
		return
	}

	h.redirect(w, r, "/next")
}

func (h *PlayerHandler) renderCheckOutForm(
	w http.ResponseWriter,
	r *http.Request,
	marker models.Marker,
	team *models.Run,
) {
	c := templates.CheckOut(marker, team.Code, team.BlockingLocation)
	if err := templates.Layout(c, "Check Out: "+marker.Name, team.Messages).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "rendering checkout", "error", err.Error())
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
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		team, err = h.runService.GetRunByCode(r.Context(), r.FormValue("run"))
		if err != nil {
			h.handleError(
				w,
				r,
				"CheckInPost: getting team by code",
				"Error finding team. Please double check your team code.",
				"error",
				err,
				"run",
				r.FormValue("run"),
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
				"run",
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
			h.logger.ErrorContext(r.Context(), "Check Out post", "err", err.Error())
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
				"run",
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
	team, err := h.getRunFromContext(r.Context())
	if err != nil || team == nil {
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "loading check ins", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// Get navigation view to determine current group settings
	view, err := h.navigationService.GetPlayerNavigationView(r.Context(), team)
	if err != nil {
		// If navigation view fails, fall back to basic rendering without view
		h.logger.ErrorContext(r.Context(), "getting navigation view", "error", err.Error())
		c := templates.MyCheckins(*team, nil)
		err = templates.Layout(c, "My Check-ins", team.Messages).Render(r.Context(), w)
		if err != nil {
			h.logger.ErrorContext(r.Context(), "rendering checkins", "error", err.Error())
		}
		return
	}

	c := templates.MyCheckins(*team, view)
	err = templates.Layout(c, "My Check-ins", team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering checkins", "error", err.Error())
	}
}
