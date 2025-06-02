package handlers

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

func (h *PublicHandler) Index(w http.ResponseWriter, r *http.Request) {
	c := templates.Index()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Home", authed).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Error rendering index", "err", err)
		return
	}
}
