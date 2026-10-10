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

	// An unchecked checkbox is absent from the form rather than false, so an
	// absent key reads as off. The page sends both toggles on every change for
	// that reason: one alone would switch the other off.
	user.CurrentQuest.Settings.EnablePoints = r.Form.Get("enablePoints") == "on"
	user.CurrentQuest.Settings.ShowLeaderboard = r.Form.Get("showLeaderboard") == "on"

	err := h.instanceSettingsService.SaveSettings(r.Context(), &user.CurrentQuest.Settings)
	if err != nil {
		h.handleError(w, r, "updating instance settings", "Error updating instance settings", "error", err)
		return
	}

	h.handleSuccess(w, r, "Settings updated")
}
