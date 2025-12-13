package public

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/public"
)

func (h *Handler) Contact(w http.ResponseWriter, r *http.Request) {
	c := templates.Contact()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Contact", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Contact: rendering template", "error", err)
	}
}

func (h *Handler) ContactPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "ContactPost: parsing form", "Error parsing form", "error", err)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	message := r.FormValue("message")

	if name == "" || email == "" || message == "" {
		h.handleError(w, r, "ContactPost: missing fields", "Please fill out all fields")
		return
	}

	err = h.emailService.SendContactEmail(r.Context(), name, email, message)
	if err != nil {
		h.handleError(w, r, "ContactPost: sending email", "Error sending email", "error", err)
		return
	}

	c := templates.ContactSuccess()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err = templates.PublicLayout(c, "Contact", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("ContactPost: rendering template", "error", err)
	}
}
