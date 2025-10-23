package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/public"
)

func (h *PublicHandler) Terms(w http.ResponseWriter, r *http.Request) {
	c := templates.Terms()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Terms and Conditions", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Error rendering terms", "err", err)
		return
	}
}
