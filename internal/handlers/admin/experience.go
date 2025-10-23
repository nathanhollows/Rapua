package admin

import (
	"maps"
	"net/http"
	"strconv"

	"github.com/nathanhollows/Rapua/v4/blocks"
	admin "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/players"
	"github.com/nathanhollows/Rapua/v4/models"
)

// Experience shows the game settings page.
func (h *Handler) Experience(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	locations, err := h.locationService.FindByInstance(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(w, r, "Experience: getting locations", "Error getting locations", "error", err)
		return
	}

	c := admin.Experience(user.CurrentInstance.Settings, len(locations))
	err = admin.Layout(c, *user, "Experience", "Experience").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering navigation page", "error", err.Error())
	}
}

// ExperiencePost handles the form submission for updating game settings.
func (h *Handler) ExperiencePost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	// Parse the navigation method
	if r.Form.Has("navigationDisplayMode") {
		method, err := models.ParseNavigationDisplayMode(r.Form.Get("navigationDisplayMode"))
		if err != nil {
			h.handleError(w, r, "Error parsing navigation method", "Error parsing navigation method", "error", err)
			return
		}
		user.CurrentInstance.Settings.NavigationDisplayMode = method
	}

	// Parse the navigation mode
	if r.Form.Has("routeStrategy") {
		mode, err := models.ParseRouteStrategy(r.Form.Get("routeStrategy"))
		if err != nil {
			h.handleError(
				w,
				r,
				"Error parsing navigation mode",
				"Error parsing navigation mode",
				"error",
				err,
				"mode",
				r.Form.Get("routeStrategy"),
			)
			return
		}
		user.CurrentInstance.Settings.RouteStrategy = mode
	}

	// Parse the completion method
	if r.Form.Has("mustCheckOut") {
		user.CurrentInstance.Settings.MustCheckOut = true
	} else {
		user.CurrentInstance.Settings.MustCheckOut = false
	}

	// Parse the maximum number of next locations
	if r.Form.Has("maxLocations") {
		maxLocations, err := strconv.Atoi(r.Form.Get("maxLocations"))
		if err != nil {
			h.handleError(w, r, "Error parsing max locations", "Error parsing max locations", "error", err)
			return
		}
		user.CurrentInstance.Settings.MaxNextLocations = maxLocations
	}

	// Parse whether to show the team count
	user.CurrentInstance.Settings.ShowTeamCount = r.Form.Has("showTeamCount") && r.Form.Get("showTeamCount") == "on"

	// Parse points
	user.CurrentInstance.Settings.EnablePoints = r.Form.Has("enablePoints") && r.Form.Get("enablePoints") == "on"
	user.CurrentInstance.Settings.EnableBonusPoints = r.Form.Has("enableBonusPoints") &&
		r.Form.Get("enableBonusPoints") == "on"

	// Update the navigation settings
	err := h.instanceSettingsService.SaveSettings(r.Context(), &user.CurrentInstance.Settings)
	if err != nil {
		h.handleError(w, r, "updating instance settings", "Error updating instance settings", "error", err)
		return
	}

	h.handleSuccess(w, r, "Settings updated")
}

// ExperiencePreview shows a preview of the next locations based on the current settings.
func (h *Handler) ExperiencePreview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	if r.Form.Has("navigationDisplayMode") {
		method, err := models.ParseNavigationDisplayMode(r.Form.Get("navigationDisplayMode"))
		if err != nil {
			h.handleError(w, r, "Error parsing navigation method", "Error parsing navigation method", "error", err)
			return
		}
		user.CurrentInstance.Settings.NavigationDisplayMode = method
	}

	if r.Form.Has("routeStrategy") {
		mode, err := models.ParseRouteStrategy(r.Form.Get("routeStrategy"))
		if err != nil {
			h.handleError(
				w,
				r,
				"Error parsing navigation mode",
				"Error parsing navigation mode",
				"error",
				err,
				"mode",
				r.Form.Get("routeStrategy"),
			)
			return
		}
		user.CurrentInstance.Settings.RouteStrategy = mode
	}

	if r.Form.Has("maxLocations") {
		user.CurrentInstance.Settings.MaxNextLocations, _ = strconv.Atoi(r.Form.Get("maxLocations"))
	}

	if r.Form.Has("showTeamCount") {
		user.CurrentInstance.Settings.ShowTeamCount = r.Form.Get("showTeamCount") == "on"
	}

	team := models.Team{
		Code:       "preview",
		InstanceID: user.CurrentInstanceID,
		Instance:   user.CurrentInstance,
	}

	locations, err := h.navigationService.GetNextLocations(r.Context(), &team)
	if err != nil {
		h.handleError(w, r, "Next: getting next locations", "Error getting next locations", "Could not load data", err)
		return
	}

	// Fetch navigation clue blocks for all next locations
	allBlocks := make([]blocks.Block, 0)
	allStates := make(map[string]blocks.PlayerState)

	if team.Instance.Settings.NavigationDisplayMode == models.NavigationDisplayCustom {
		for _, location := range locations {
			locationBlocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
				r.Context(),
				location.ID,
				team.Code,
				blocks.ContextLocationClues,
			)
			if err != nil {
				h.handleError(
					w,
					r,
					"Next: getting navigation blocks",
					"Error loading navigation clues",
					"Could not load navigation clues",
					err,
				)
				return
			}
			allBlocks = append(allBlocks, locationBlocks...)
			maps.Copy(allStates, blockStates)
		}
	}

	var nextData templates.NextParams
	nextData.Team = team
	nextData.Settings = team.Instance.Settings
	nextData.Locations = locations
	nextData.Blocks = allBlocks
	nextData.States = allStates

	// data["notifications"], _ = h.NotificationService.GetNotifications(r.Context(), team.Code)
	err = templates.Next(nextData).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering template", "error", err)
	}
}
