package public

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/public"
)

func (h *Handler) TemplatesPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	shareLink, err := h.templateService.GetShareLink(r.Context(), id)
	if err != nil {
		h.handleError(w, r, "TemplatesPreview: getting template", "Error getting template", "error", err, "id", id)
		return
	}

	if shareLink.IsExpired() {
		authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
		c := templates.TemplateNotFound()
		err = templates.PublicLayout(c, "Template not found", authed).Render(r.Context(), w)
		if err != nil {
			h.logger.ErrorContext(r.Context(), "Contact: rendering template", "error", err)
		}
		return
	}

	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	c := templates.TemplatePeview(*shareLink, authed)
	err = templates.PublicLayout(c, "Template: "+shareLink.Template.Name, authed).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Contact: rendering template", "error", err)
	}
}
