package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

func (h *PublicHandler) TemplatesPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	shareLink, err := h.TemplateService.GetShareLink(r.Context(), id)
	if err != nil {
		h.handleError(w, r, "TemplatesPreview: getting template", "Error getting template", "error", err, "id", id)
		return
	}

	c := templates.TemplatePeview(*shareLink)
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err = templates.PublicLayout(c, "Template", authed).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Contact: rendering template", "error", err)
	}
}
