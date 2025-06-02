package handlers

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
	p "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

func (h *AdminHandler) Plan(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.Plan(user.Email)
	err := p.AuthLayout(c, "Payment", true).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Payment: rendering template", "error", err)
	}
}

func (h *AdminHandler) PlanPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	r.ParseForm()

	c := templates.Plan(user.Email)
	err := p.AuthLayout(c, "Payment", true).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Payment: rendering template", "error", err)
	}
}
