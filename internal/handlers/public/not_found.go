package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

// NotFound shows the not found page.
func (h *PublicHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	c := templates.NotFound()
	err := templates.PublicLayout(c, "Not Found").Render(r.Context(), w)

	if err != nil {
		h.Logger.Error("rendering NotFound page", "err", err)
	}
}
