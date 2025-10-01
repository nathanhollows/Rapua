package players

import (
	"net/http"

	"github.com/go-chi/chi"
)

// DismissNotificationPost dismisses a message.
func (h *PlayerHandler) DismissNotificationPost(w http.ResponseWriter, r *http.Request) {
	notificationID := chi.URLParam(r, "ID")
	err := h.notificationService.DismissNotification(r.Context(), notificationID)

	// Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		if err != nil {
			h.logger.Error("dismissing notification", "error", err.Error(), "notificationID", notificationID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if err != nil {
		h.logger.Error("dismissing notification", "error", err.Error(), "notificationID", notificationID)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)

		return
	}

	http.Redirect(w, r, "/play", http.StatusSeeOther)
}
