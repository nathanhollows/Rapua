package admin

import (
	"net/http"

	admin "github.com/nathanhollows/Rapua/v6/internal/templates/admin"
)

// NotifyAllPost sends a notification to all teams.
func (h *Handler) NotifyAllPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"NotifyAllPost parsing form",
			"Error parsing form",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	content := r.FormValue("content")

	// Send the notification
	err := h.notificationService.SendNotificationToAllTeams(r.Context(), user.CurrentInstanceID, content)
	if err != nil {
		h.handleError(
			w,
			r,
			"NotifyAllPost sending notification",
			"Error sending notification",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	h.handleSuccess(w, r, "Notification sent")
}

// NotifyTeamPost sends a notification to a specific team.
func (h *Handler) NotifyTeamPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"NotifyTeamPost parsing form",
			"Error parsing form",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	content := r.FormValue("content")
	teamCode := r.FormValue("teamCode")

	// Send the notification
	_, err := h.notificationService.SendNotification(r.Context(), teamCode, content)
	if err != nil {
		h.handleError(
			w,
			r,
			"NotifyTeamPost sending notification",
			"Error sending notification",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Fetch updated notifications
	notifications, err := h.notificationService.GetNotifications(r.Context(), teamCode)
	if err != nil {
		h.handleError(
			w,
			r,
			"NotifyTeamPost fetching notifications",
			"Error fetching notifications",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Render the updated alerts list
	err = admin.AlertsList(notifications).Render(r.Context(), w)
	if err != nil {
		h.handleError(
			w,
			r,
			"NotifyTeamPost rendering alerts list",
			"Error rendering notifications",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}
}
