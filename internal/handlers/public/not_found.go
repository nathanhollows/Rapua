package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/public"
)

// NotFound shows the not found page.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	c := templates.NotFound()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Not Found", authed).Render(r.Context(), w)

	if err != nil {
		h.logger.Error("rendering NotFound page", "err", err)
	}
}
