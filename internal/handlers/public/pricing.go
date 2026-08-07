package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/public"
)

// Pricing shows the pricing page.
func (h *Handler) Pricing(w http.ResponseWriter, r *http.Request) {
	c := templates.Pricing()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Pricing", authed).Render(r.Context(), w)

	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering Pricing page", "err", err)
	}
}
