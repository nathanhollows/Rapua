package public

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/public"
	"github.com/nathanhollows/Rapua/v8/models"
)

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	// A closed quest has nothing to resume.
	run, _ := r.Context().Value(contextkeys.RunKey).(*models.Run)
	if run != nil && run.Quest.GetStatus() == models.Closed {
		run = nil
	}
	h.renderPage(w, r, "home", func(authed bool) templ.Component {
		return templates.Home(run, services.RunCodeLength, authed)
	})
}
