package admin

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
)

// Quickstart shows the quickstart bar.
func (h *Handler) Quickstart(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.QuickstartBar(user.CurrentQuest).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Quickstart: rendering template", "error", err)
	}
}

// DismissQuickstart dismisses the quickstart.
func (h *Handler) DismissQuickstart(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.quickstartService.DismissQuickstart(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.handleError(w, r, "DismissQuickstart", "Error dismissing quickstart", "error", err)
		return
	}

	if r.URL.Query().Has("redirect") {
		h.redirect(w, r, "/admin/activity")
	}
}
