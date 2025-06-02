package handlers

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
)

func (h *PublicHandler) Contact(w http.ResponseWriter, r *http.Request) {
	c := templates.Contact()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err := templates.PublicLayout(c, "Contact", authed).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Contact: rendering template", "error", err)
	}
}

func (h *PublicHandler) ContactPost(w http.ResponseWriter, r *http.Request) {
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

	_, err = h.EmailService.SendContactEmail(r.Context(), name, email, message)
	if err != nil {
		h.handleError(w, r, "ContactPost: sending email", "Error sending email", "error", err)
		return
	}

	c := templates.ContactSuccess()
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err = templates.PublicLayout(c, "Contact", authed).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("ContactPost: rendering template", "error", err)
	}
}
