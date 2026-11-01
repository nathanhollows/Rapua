package public

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/public"
)

// Creators shows the page for people who build quests.
func (h *Handler) Creators(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "creators", templates.CreatorsPage)
}
