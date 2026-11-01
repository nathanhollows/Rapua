package public

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/public"
)

func (h *Handler) About(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "about", templates.AboutPage)
}
