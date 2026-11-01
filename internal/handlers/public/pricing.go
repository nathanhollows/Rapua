package public

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/public"
)

// Pricing shows the pricing page.
func (h *Handler) Pricing(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "pricing", templates.PricingPage)
}
