package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/public"
)

func (h *Handler) Terms(w http.ResponseWriter, r *http.Request) {
	c := templates.Terms()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Terms and Conditions", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Error rendering terms", "err", err)
		return
	}
}
