package public

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
)

// renderPage renders a public page that needs only to know whether an admin is
// signed in, which the header uses to offer Admin rather than Admin login.
func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, name string, page func(authed bool) templ.Component) {
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	if err := page(authed).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "rendering "+name+" page", "err", err)
	}
}
