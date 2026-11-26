package public

import (
	"net/http"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/public"
)

func (h *Handler) Contact(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "contact", templates.ContactPage)
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

	err = templates.ContactSuccess().Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ContactPost: rendering template", "error", err)
	}
}
