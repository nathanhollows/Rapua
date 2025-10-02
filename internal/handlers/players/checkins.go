package players

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v4/internal/flash"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/players"
	"github.com/nathanhollows/Rapua/v4/models"
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

// CheckInView shows the page for a specific location.
func (h *PlayerHandler) CheckInView(w http.ResponseWriter, r *http.Request) {
	// If the user is in preview mode, show the preview
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.checkInPreview(w, r)
		return
	}

	locationCode := chi.URLParam(r, "id")

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.logger.Error("loading team", "error", err.Error())
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	var index int
	err = h.teamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.Error("loading team relations", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// Get the index of the location in the team's scans
	index = -1
	for i, scan := range team.CheckIns {
		if scan.Location.MarkerID == locationCode {
			index = i
			break
		}
	}

	if index == -1 {
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	blocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithState(
		r.Context(),
		team.CheckIns[index].Location.ID,
		team.Code,
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"CheckInView: getting blocks",
			"Error loading blocks",
			"error",
			err,
			"team",
			team.Code,
			"location",
			locationCode,
		)
		return
	}

	c := templates.CheckInView(team.Instance.Settings, team.CheckIns[index], blocks, blockStates)
	err = templates.Layout(c, team.CheckIns[index].Location.Name, team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering checkin view", "error", err.Error())
	}
}

// checkInPreview shows a player preview of the given location.
func (h *PlayerHandler) checkInPreview(w http.ResponseWriter, r *http.Request) {
	locationCode := chi.URLParam(r, "id")

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "LocationPreview: getting team", "Error getting team", "error", err)
		return
	}

	err = h.teamService.LoadRelation(r.Context(), team, "Instance")
	if err != nil {
		h.handleError(w, r, "LocationPreview: loading instance", "Error loading instance", "error", err)
		return
	}

	var location models.Location
	for _, loc := range team.Instance.Locations {
		if loc.MarkerID == locationCode {
			location = loc
			break
		}
	}
	if location.MarkerID == "" {
		h.handleError(w, r, "LocationPreview: finding location", "Location not found", "error", "Location not found")
		return
	}

	scan := models.CheckIn{
		Location: location,
	}

	contentBlocks, err := h.blockService.FindByOwnerID(r.Context(), location.ID)
	if err != nil {
		h.handleError(w, r, "LocationPreview: getting blocks", "Error getting blocks", "error", err)
		return
	}

	blockStates := make(map[string]blocks.PlayerState, len(contentBlocks))
	for _, block := range contentBlocks {
		blockStates[block.GetID()], err = h.blockService.NewMockBlockState(r.Context(), block.GetID(), "")
		if err != nil {
			h.handleError(w, r, "LocationPreview: creating block state", "Error creating block state", "error", err)
			return
		}
	}

	err = templates.CheckInView(team.Instance.Settings, scan, contentBlocks, blockStates).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("LocationPreview: rendering template", "error", err)
	}
}
