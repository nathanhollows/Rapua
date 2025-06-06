package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
)

// Settings displays the account settings page.
func (h *AdminHandler) Settings(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Settings(*user)
	err := templates.Layout(c, *user, "Settings", "Account Settings").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsProfile displays the account profile page.
func (h *AdminHandler) SettingsProfile(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.SettingsProfile(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsProfile displays the account profile page.
func (h *AdminHandler) SettingsProfilePost(w http.ResponseWriter, r *http.Request) {
	_ = h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "SettingsProfilePost: parse form", "Failed to parse form data", err)
		return
	}

	h.handleSuccess(w, r, "Updated!")
}

// SettingsAppearance displays the account appearance settings page.
func (h *AdminHandler) SettingsAppearance(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.SettingsAppearance(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account appearance page", "error", err.Error())
	}
}

// SettingsSecurity displays the account security settings page.
func (h *AdminHandler) SettingsSecurity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.SettingsSecurity(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account security page", "error", err.Error())
	}
}

// SettingsBilling displays the account billing settings page.
func (h *AdminHandler) SettingsBilling(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.SettingsBilling(*user).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering account billing page", "error", err.Error())
	}
}
