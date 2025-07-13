package handlers

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/players"
)

func (h *PlayerHandler) Next(w http.ResponseWriter, r *http.Request) {
	preview := r.Context().Value(contextkeys.PreviewKey) != nil

	team, err := h.getTeamFromContext(r.Context())
	if err != nil && !preview {
		h.redirect(w, r, "/play")
		return
	}

	locations, err := h.NavigationService.GetNextLocations(r.Context(), team)
	if err != nil && !preview {
		if errors.Is(err, services.ErrAllLocationsVisited) && team.MustCheckOut == "" {
			h.redirect(w, r, "/finish")
			return
		}
		h.handleError(w, r, "Next: getting next locations", "Error getting next locations", "Could not load data", err)
		return
	}

	// If the user is in preview mode, only render the template, not the full layout.
	template := templates.Next(*team, locations)
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		template = templates.Layout(template, "Next stops", team.Messages)
	}
	err = template.Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Next: rendering template", "Error rendering template", "Could not render template", err)
	}
}
