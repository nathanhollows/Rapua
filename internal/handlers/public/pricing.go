package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/public"
)

// Pricing shows the pricing page.
func (h *PublicHandler) Pricing(w http.ResponseWriter, r *http.Request) {
	c := templates.Pricing()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Pricing", authed).Render(r.Context(), w)

	if err != nil {
		h.logger.Error("rendering Pricing page", "err", err)
	}
}
