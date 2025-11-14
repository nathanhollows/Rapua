package admin

import (
	"net/http"

	admin "github.com/nathanhollows/Rapua/v6/internal/templates/admin"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
	"github.com/nathanhollows/Rapua/v6/models"
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

	// Parse the completion method
	if r.Form.Has("mustCheckOut") {
		user.CurrentInstance.Settings.MustCheckOut = true
	} else {
		user.CurrentInstance.Settings.MustCheckOut = false
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

	if r.Form.Has("showTeamCount") {
		user.CurrentInstance.Settings.ShowTeamCount = r.Form.Get("showTeamCount") == "on"
	}

	team := models.Team{
		Code:       "preview",
		InstanceID: user.CurrentInstanceID,
		Instance:   user.CurrentInstance,
	}

	// Get complete navigation view from service
	view, err := h.navigationService.GetPlayerNavigationView(r.Context(), &team)
	if err != nil {
		h.handleError(w, r, "Next: getting navigation view", "Error loading navigation", "Could not load data", err)
		return
	}

	nextData := templates.NextParams{
		Team: team,
		View: view,
	}

	err = templates.Next(nextData).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering template", "error", err)
	}
}
