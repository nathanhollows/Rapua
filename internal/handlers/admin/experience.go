package handlers

import (
	"net/http"
	"strconv"

	admin "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
	players "github.com/nathanhollows/Rapua/v3/internal/templates/players"
	"github.com/nathanhollows/Rapua/v3/models"
)

// Show the form to edit the navigation settings.
func (h *AdminHandler) Experience(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	locations, err := h.LocationService.FindByInstance(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(w, r, "Experience: getting locations", "Error getting locations", "error", err)
		return
	}

	c := admin.Experience(user.CurrentInstance.Settings, len(locations))
	err = admin.Layout(c, *user, "Experience", "Experience").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering navigation page", "error", err.Error())
	}
}

// Update the navigation settings.
func (h *AdminHandler) ExperiencePost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	// Parse the navigation method
	if !r.Form.Has("navigationMethod") {
		method, err := models.ParseNavigationMethod(r.Form.Get("navigationMethod"))
		if err != nil {
			h.handleError(w, r, "Error parsing navigation method", "Error parsing navigation method", "error", err)
			return
		}
		user.CurrentInstance.Settings.NavigationMethod = method
	}

	// Parse the navigation mode
	if !r.Form.Has("navigationMode") {
		mode, err := models.ParseNavigationMode(r.Form.Get("navigationMode"))
		if err != nil {
			h.handleError(w, r, "Error parsing navigation mode", "Error parsing navigation mode", "error", err, "mode", r.Form.Get("navigationMode"))
			return
		}
		user.CurrentInstance.Settings.NavigationMode = mode
	}

	// Parse the completion method
	if r.Form.Has("completionMethod") {
		completionMethod, err := models.ParseCompletionMethod(r.Form.Get("completionMethod"))
		if err != nil {
			h.handleError(w, r, "Error parsing completion method", "Error parsing completion method", "error", err)
			return
		}
		user.CurrentInstance.Settings.CompletionMethod = completionMethod
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
	if r.Form.Has("showTeamCount") {
		showTeamCount := r.Form.Get("showTeamCount") == "on"
		user.CurrentInstance.Settings.ShowTeamCount = showTeamCount
	}

	// Parse points
	user.CurrentInstance.Settings.EnablePoints = r.Form.Has("enablePoints")
	user.CurrentInstance.Settings.EnableBonusPoints = r.Form.Has("enableBonusPoints")

	// Update the navigation settings
	err := h.instanceSettingsService.SaveSettings(r.Context(), &user.CurrentInstance.Settings)
	if err != nil {
		h.handleError(w, r, "updating instance settings", "Error updating instance settings", "error", err)
		return
	}

	h.handleSuccess(w, r, "Settings updated")
}

// Show a player preview for navigation.
func (h *AdminHandler) ExperiencePreview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	if r.Form.Has("navigationMethod") {
		method, err := models.ParseNavigationMethod(r.Form.Get("navigationMethod"))
		if err != nil {
			h.handleError(w, r, "Error parsing navigation method", "Error parsing navigation method", "error", err)
			return
		}
		user.CurrentInstance.Settings.NavigationMethod = method
	}

	if r.Form.Has("navigationMode") {
		mode, err := models.ParseNavigationMode(r.Form.Get("navigationMode"))
		if err != nil {
			h.handleError(w, r, "Error parsing navigation mode", "Error parsing navigation mode", "error", err, "mode", r.Form.Get("navigationMode"))
			return
		}
		user.CurrentInstance.Settings.NavigationMode = mode
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

	locations, err := h.NavigationService.GetNextLocations(r.Context(), &team)
	if err != nil {
		h.handleError(w, r, "Next: getting next locations", "Error getting next locations", "Could not load data", err)
		return
	}

	// data["notifications"], _ = h.NotificationService.GetNotifications(r.Context(), team.Code)
	err = players.Next(team, locations).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering template", "error", err)
	}
}
