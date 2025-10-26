package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v5/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v5/internal/templates/public"
)

func (h *PublicHandler) Index(w http.ResponseWriter, r *http.Request) {
	c := templates.Index()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Home", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Error rendering index", "err", err)
		return
	}
}
