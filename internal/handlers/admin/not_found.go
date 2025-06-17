package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
)

// NotFound shows the not found page.
func (h *AdminHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	w.Header().Set("Content-Type", "text/html")

	if r.Header.Get("HX-Boosted") != "true" {
		h.Logger.Warn("NotFound called without HTMX boost", "path", r.URL.Path)
		err := templates.NotFound().Render(r.Context(), w)
		if err != nil {
			h.Logger.Error("rendering NotFound page", "err", err.Error())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	c := templates.NotFound()
	err := templates.Layout(c, *user, "Error", "Not Found").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering NotFound page", "error", err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
