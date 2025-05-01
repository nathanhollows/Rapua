package handlers

import (
	"errors"
	"net/http"
	"os"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
	"github.com/nathanhollows/Rapua/v3/services"
)

func (h *PublicHandler) Docs(w http.ResponseWriter, r *http.Request) {
	docsService, err := services.NewDocsService("./docs")
	if err != nil {
		h.Logger.Error("Docs: creating docs service", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Extract the path after /docs/
	path := r.URL.Path
	if path == "/docs" || path == "/docs/" {
		path = "/docs/index"
	}

	page, err := docsService.GetPage(path)
	if err == nil {
		// Page found, render it
	} else if errors.Is(err, os.ErrNotExist) {
		h.NotFound(w, r)
		return
	} else {
		var redirectErr *services.RedirectError
		if errors.As(err, &redirectErr) {
			h.redirect(w, r, redirectErr.RedirectTo)
			return
		}

		// Any other error
		h.handleError(w, r, "Docs: getting page", "Error getting page", "error", err)
		return
	}
	c := templates.Docs(page, docsService.Pages)
	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	err = templates.PublicLayout(c, page.Title+" - Docs", authed).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Contact: rendering template", "error", err)
	}
}
