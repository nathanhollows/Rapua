package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
)

// Settings displays the account settings page.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Settings(templates.SettingsProfile(*user))
	err := templates.Layout(c, *user, "Settings", "Profile").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsProfile displays the account profile page.
func (h *Handler) SettingsProfile(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Settings(templates.SettingsProfile(*user))
	err := templates.Layout(c, *user, "Settings", "Appearance").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsProfilePost handles updating the user's profile settings.
func (h *Handler) SettingsProfilePost(w http.ResponseWriter, r *http.Request) {
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
	err = h.userService.UpdateUserProfile(r.Context(), user, profileData)
	if err != nil {
		h.handleError(w, r, "SettingsProfilePost: update user", "Failed to update user profile", err)
		return
	}

	h.handleSuccess(w, r, "Profile updated successfully!")
}

// SettingsAppearance displays the account appearance settings page.
func (h *Handler) SettingsAppearance(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Settings(templates.SettingsAppearance(*user))
	err := templates.Layout(c, *user, "Settings", "Appearance").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsSecurity displays the account security settings page.
func (h *Handler) SettingsSecurity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Settings(templates.SettingsSecurity(*user))
	err := templates.Layout(c, *user, "Settings", "Security").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsBilling displays the account billing settings page.
func (h *Handler) SettingsBilling(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := templates.SettingsBilling(*user).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering account billing page", "error", err.Error())
	}
}

// SettingsSecurityPost handles updating security settings like password.
func (h *Handler) SettingsSecurityPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "SettingsSecurityPost: parse form", "Failed to parse form data", err)
		return
	}

	// Handle password change if that's what was submitted
	oldPassword := r.FormValue("old_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if oldPassword != "" || newPassword != "" || confirmPassword != "" {
		// Check if all password fields are provided
		if oldPassword == "" || newPassword == "" || confirmPassword == "" {
			h.handleError(w, r, "SettingsSecurityPost", "All password fields are required", nil)
			return
		}

		// Call the service to change the password
		changeErr := h.userService.ChangePassword(r.Context(), user, oldPassword, newPassword, confirmPassword)
		if changeErr != nil {
			var errorMessage string
			switch {
			case errors.Is(changeErr, services.ErrIncorrectOldPassword):
				errorMessage = "Current password is incorrect"
			case errors.Is(changeErr, services.ErrPasswordsDoNotMatch):
				errorMessage = "New passwords do not match"
			case errors.Is(changeErr, services.ErrEmptyPassword):
				errorMessage = "Password cannot be empty"
			default:
				errorMessage = "Failed to update password"
				h.logger.Error("change password", "error", changeErr.Error())
			}

			h.handleError(w, r, "SettingsSecurityPost", errorMessage, changeErr)
			return
		}

		h.handleSuccess(w, r, "Password updated successfully!")
		return
	}

	h.handleSuccess(w, r, "Security settings updated!")
}

// DeleteAccount handles account deletion.
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
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

	err = h.deleteService.DeleteUser(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, "DeleteAccount", "Failed to delete account", err)
		return
	}

	h.redirect(w, r, "/logout")
}

// SettingsCreditUsage displays the user's credit usage data.
func (h *Handler) SettingsCreditUsage(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	var recurring = 10
	if user.IsEducator {
		recurring = 50
	}

	topupFilter := services.CreditAdjustmentFilter{
		UserID: user.ID,
		Limit:  25,
		Offset: 0,
	}
	topups, err := h.creditService.GetCreditAdjustments(r.Context(), topupFilter)
	if err != nil {
		h.handleError(w, r, "SettingsCreditUsage: get credit adjustments", "Failed to retrieve credit adjustments", err)
		return
	}

	usageFilter := services.TeamStartLogFilter{
		UserID:    user.ID,
		StartTime: time.Now().AddDate(0, 0, -6), // Last year
		EndTime:   time.Now(),
		GroupBy:   "day",
	}
	usage, err := h.creditService.GetTeamStartLogsSummary(r.Context(), usageFilter)
	if err != nil {
		h.handleError(w, r, "SettingsCreditUsage: get team start logs", "Failed to retrieve team start logs", err)
		return
	}

	c := templates.Settings(templates.SettingsCreditUsage(
		user.FreeCredits,
		user.PaidCredits,
		recurring,
		topups,
		usage,
	))
	err = templates.Layout(c, *user, "Settings", "Credit Usage").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering account page", "error", err.Error())
	}
}

// SettingsCreditUsageChart displays the credit usage chart data.
func (h *Handler) SettingsCreditUsageChart(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "SettingsCreditUsageChart: parse form", "Failed to parse form data", err)
		return
	}

	var start, end time.Time
	groupBy := "day"
	switch r.FormValue("period") {
	case "week":
		start = time.Now().AddDate(0, 0, -6) // Last week
		end = time.Now()
	case "month":
		start = time.Now().AddDate(0, -1, 0) // Last month
		end = time.Now()
	case "year":
		start = time.Now().AddDate(-1, 1, 0) // Last year
		end = time.Now()
		groupBy = "month"
	default:
		h.handleError(w, r, "SettingsCreditUsageChart", "Invalid period specified", nil)
		return
	}

	usageFilter := services.TeamStartLogFilter{
		UserID:    user.ID,
		StartTime: start,
		EndTime:   end,
		GroupBy:   groupBy,
	}

	usage, err := h.creditService.GetTeamStartLogsSummary(r.Context(), usageFilter)
	if err != nil {
		h.handleError(w, r, "SettingsCreditUsageChart: get team start logs", "Failed to retrieve team start logs", err)
		return
	}

	err = templates.CreditUsageChart(usage, r.FormValue("period")).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "SettingsCreditUsageChart: render chart", "Failed to render credit usage chart", err)
		return
	}
}
