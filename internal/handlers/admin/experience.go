package handlers

import (
	"net/http"
	"strconv"

	"github.com/nathanhollows/Rapua/internal/services"
	admin "github.com/nathanhollows/Rapua/internal/templates/admin"
	players "github.com/nathanhollows/Rapua/internal/templates/players"
	"github.com/nathanhollows/Rapua/models"
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
		w.WriteHeader(http.StatusInternalServerError)
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	// Update the navigation settings
	response := h.GameManagerService.UpdateSettings(r.Context(), &user.CurrentInstance.Settings, r.Form)
	if response.Error != nil {
		w.WriteHeader(http.StatusExpectationFailed)
		h.handleError(w, r, "updating instance settings", "Error updating instance settings", "error", response.Error)
		return
	}

	err := admin.Toast(response.FlashMessages...).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering toast", "error", err.Error())
	}
}

// Show a player preview for navigation
func (h *AdminHandler) ExperiencePreview(w http.ResponseWriter, r *http.Request) {
	gameplayService := services.NewGameplayService()
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

	locations, err := gameplayService.SuggestNextLocations(r.Context(), &team)
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
