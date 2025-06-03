package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
)

// Account displays the account settings page.
func (h *AdminHandler) Account(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Account(*user)
	err := templates.Layout(c, *user, "Account", "Account Settings").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account page", "error", err.Error())
	}
}
