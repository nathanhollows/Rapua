package players

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/flash"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

// ScanLink is /s/{code}: a printed code opened by the phone's own camera. The
// path is v7's, so posters already on walls keep working.
//
// No match lands on the list rather than an error page: a code for something
// still locked is an ordinary moment in play, and the list says what is open.
func (h *PlayerHandler) ScanLink(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/play", http.StatusSeeOther)
		return
	}

	found, err := h.checkInService.ScanCode(r.Context(), team, chi.URLParam(r, "code"), blocks.ModalityCamera)
	switch {
	case errors.Is(err, services.ErrNoScanMatch):
		http.Redirect(w, r, "/objectives", http.StatusSeeOther)
	case err != nil:
		h.logger.ErrorContext(r.Context(), "ScanLink: matching code", "error", err, "run", team.Code)
		http.Redirect(w, r, "/objectives", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/objective/"+url.PathEscape(found.Slug), http.StatusSeeOther)
	}
}

// ScanPost takes a value from the in-app scanner or a typed code.
func (h *PlayerHandler) ScanPost(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}
	if err = r.ParseForm(); err != nil {
		h.handleError(w, r, "ScanPost: parsing form", "Could not read that code", "error", err)
		return
	}

	value, modality := strings.TrimSpace(r.Form.Get("scanned")), blocks.ModalityCamera
	if value == "" {
		value, modality = strings.TrimSpace(r.Form.Get("code")), blocks.ModalityTyped
	}
	if value == "" {
		h.toast(w, r, *flash.NewWarning("Scan a code or type it in."))
		return
	}

	found, err := h.checkInService.ScanCode(r.Context(), team, value, modality)
	switch {
	case errors.Is(err, services.ErrNoScanMatch):
		h.toast(w, r, *flash.NewWarning("That code doesn't match anything you can reach yet."))
	case err != nil:
		h.handleError(w, r, "ScanPost: matching code", "Could not check that code", "error", err, "run", team.Code)
	default:
		h.redirect(w, r, "/objective/"+url.PathEscape(found.Slug))
	}
}

func (h *PlayerHandler) toast(w http.ResponseWriter, r *http.Request, message flash.Message) {
	if err := templates.Toast(message).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "rendering toast", "error", err)
	}
}
