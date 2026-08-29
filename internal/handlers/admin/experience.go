package admin

import (
	"net/http"

	admin "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
)

// Experience shows the game settings page.
func (h *Handler) Experience(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := admin.Experience(user.CurrentQuest.Settings)
	err := admin.Layout(c, *user, "Experience", "Experience").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering navigation page", "error", err.Error())
	}
}

// ExperiencePost handles the form submission for updating game settings.
func (h *Handler) ExperiencePost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	// Parse whether to show the team count
	user.CurrentQuest.Settings.ShowTeamCount = r.Form.Has("showTeamCount") && r.Form.Get("showTeamCount") == "on"

	// Parse points
	user.CurrentQuest.Settings.EnablePoints = r.Form.Has("enablePoints") && r.Form.Get("enablePoints") == "on"

	// Update the navigation settings
	err := h.instanceSettingsService.SaveSettings(r.Context(), &user.CurrentQuest.Settings)
	if err != nil {
		h.handleError(w, r, "updating instance settings", "Error updating instance settings", "error", err)
		return
	}

	h.handleSuccess(w, r, "Settings updated")
}
