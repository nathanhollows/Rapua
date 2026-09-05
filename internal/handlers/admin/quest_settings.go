package admin

import (
	"net/http"
)

// QuestSettingsPost handles the quest page's own settings controls, which are
// each a single toggle rather than a form worth a page.
func (h *Handler) QuestSettingsPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	// Parse points
	user.CurrentQuest.Settings.EnablePoints = r.Form.Has("enablePoints") && r.Form.Get("enablePoints") == "on"

	err := h.instanceSettingsService.SaveSettings(r.Context(), &user.CurrentQuest.Settings)
	if err != nil {
		h.handleError(w, r, "updating instance settings", "Error updating instance settings", "error", err)
		return
	}

	h.handleSuccess(w, r, "Settings updated")
}
