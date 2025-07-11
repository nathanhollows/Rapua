package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
)

// Quickstart shows the quickstart bar.
func (h *AdminHandler) Quickstart(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.QuickstartBar(user.CurrentInstance).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Quickstart: rendering template", "error", err)
	}
}

// DismissQuickstart dismisses the quickstart.
func (h *AdminHandler) DismissQuickstart(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.QuickstartService.DismissQuickstart(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(w, r, "DismissQuickstart", "Error dismissing quickstart", "error", err)
		return
	}

	if r.URL.Query().Has("redirect") {
		h.redirect(w, r, "/admin/activity")
	}
}
