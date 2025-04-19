package handlers

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

// Pricing shows the pricing page.
func (h *PublicHandler) Pricing(w http.ResponseWriter, r *http.Request) {
	c := templates.Pricing()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Pricing", authed).Render(r.Context(), w)

	if err != nil {
		h.Logger.Error("rendering Pricing page", "err", err)
	}
}
