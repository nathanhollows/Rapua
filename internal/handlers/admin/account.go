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

// AccountProfile displays the account profile page.
func (h *AdminHandler) AccountProfile(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	
	err := templates.AccountProfile(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account page", "error", err.Error())
	}
}

// AccountAppearance displays the account appearance settings page.
func (h *AdminHandler) AccountAppearance(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.AccountAppearance(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account appearance page", "error", err.Error())
	}
}

// AccountSecurity displays the account security settings page.
func (h *AdminHandler) AccountSecurity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.AccountSecurity(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account security page", "error", err.Error())
	}
}

// AccountBilling displays the account billing settings page.
func (h *AdminHandler) AccountBilling(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.AccountBilling(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account billing page", "error", err.Error())
	}
}
