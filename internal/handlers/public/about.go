package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

func (h *PublicHandler) About(w http.ResponseWriter, r *http.Request) {
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	c := templates.About()
	err := templates.PublicLayout(c, "About", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Error rendering index", "err", err)
		return
	}
}
