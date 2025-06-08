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

// SettingsProfilePost handles updating the user's profile settings
func (h *AdminHandler) SettingsProfilePost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "SettingsProfilePost: parse form", "Failed to parse form data", err)
		return
	}

	// Create a map of form values
	profileData := map[string]string{
		"name":            r.FormValue("name"),
		"display_name":    r.FormValue("display_name"),
		"work_type":       r.FormValue("work_type"),
		"other_work_type": r.FormValue("other_work_type"),
		"theme":           r.FormValue("theme"),
		"show_email":      r.FormValue("show_email"),
	}

	// Update the user in the database using the service
	err = h.UserService.UpdateUserProfile(r.Context(), user, profileData)
	if err != nil {
		h.handleError(w, r, "SettingsProfilePost: update user", "Failed to update user profile", err)
		return
	}

	h.handleSuccess(w, r, "Profile updated successfully!")
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

// SettingsSecurityPost handles updating security settings like password
func (h *AdminHandler) SettingsSecurityPost(w http.ResponseWriter, r *http.Request) {
	// Unused for now, but will be used for password verification
	// user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "SettingsSecurityPost: parse form", "Failed to parse form data", err)
		return
	}

	// Handle password change if that's what was submitted
	oldPassword := r.FormValue("old_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if oldPassword != "" && newPassword != "" && confirmPassword != "" {
		// Password handling logic would go here
		// This is a placeholder for the actual implementation
		if newPassword != confirmPassword {
			h.handleError(w, r, "SettingsSecurityPost", "New passwords do not match", nil)
			return
		}

		// You would add password verification and update logic here

		h.handleSuccess(w, r, "Password updated successfully!")
		return
	}

	h.handleSuccess(w, r, "Security settings updated!")
}

// DeleteAccount handles account deletion
func (h *AdminHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "DeleteAccount: parse form", "Failed to parse form data", err)
		return
	}
	// Confirm deletion
	confirm := r.FormValue("confirm-email")
	if confirm != user.Email {
		h.handleError(w, r, "DeleteAccount", "Email confirmation does not match", nil)
		return
	}

	err = h.UserService.DeleteUser(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, "DeleteAccount", "Failed to delete account", err)
		return
	}

	h.redirect(w, r, "/logout")
}
