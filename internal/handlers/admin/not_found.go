package admin

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
)

// NotFound shows the not found page, in the admin's own frame.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	if user == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// htmx will not swap a 4xx into the page, so a boosted visit has to say 200 to be
	// seen at all. Anything else is an address that really is not found.
	if r.Header.Get("Hx-Boosted") != htmxHeaderTrue {
		w.WriteHeader(http.StatusNotFound)
	}

	err := templates.Layout(templates.NotFound(), *user, "Error", "Not Found").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering NotFound page", "error", err.Error())
	}
}
