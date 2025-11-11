package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/public"
)

func (h *PublicHandler) Privacy(w http.ResponseWriter, r *http.Request) {
	c := templates.Privacy()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Privacy", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Error rendering index", "err", err)
		return
	}
}
