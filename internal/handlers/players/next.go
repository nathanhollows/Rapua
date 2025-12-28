package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
)

func (h *PlayerHandler) Next(w http.ResponseWriter, r *http.Request) {
	// If the user is in preview mode, show the preview
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.nextPreview(w, r)
		return
	}

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	// Get complete navigation view from service
	view, err := h.navigationService.GetPlayerNavigationView(r.Context(), team)
	if err != nil {
		if errors.Is(err, services.ErrAllLocationsVisited) && !view.MustCheckOut {
			h.redirect(w, r, "/complete")
			return
		}
		h.handleError(w, r, "Next: getting navigation view", "Error loading navigation", "Could not load data", err)
		return
	}

	nextData := templates.NextParams{
		Team: *team,
		View: view,
	}

	template := templates.Next(nextData)
	template = templates.Layout(template, "Next stops", team.Messages)
	err = template.Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Next: rendering template", "Error rendering template", "Could not render template", err)
	}
}

func (h *PlayerHandler) nextPreview(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "NextPreview: getting team", "Error getting team", "error", err)
		return
	}

	err = h.teamService.LoadRelation(r.Context(), team, "Instance")
	if err != nil {
		h.handleError(w, r, "NextPreview: loading instance", "Error loading instance", "error", err)
		return
	}

	var view *services.PlayerNavigationView

	// Check for location_id query param to preview a specific location
	targetLocationID := r.URL.Query().Get("location_id")
	if targetLocationID != "" {
		view, err = h.navigationService.GetPreviewNavigationView(r.Context(), team, targetLocationID)
		if err != nil {
			h.handleError(w, r, "NextPreview: building location view", "Error loading location", "error", err)
			return
		}
	} else {
		// No specific location - preview first location in first group
		if len(team.Instance.GameStructure.SubGroups) > 0 && len(team.Instance.GameStructure.SubGroups[0].LocationIDs) > 0 {
			firstLocationID := team.Instance.GameStructure.SubGroups[0].LocationIDs[0]
			view, err = h.navigationService.GetPreviewNavigationView(r.Context(), team, firstLocationID)
			if err != nil {
				h.handleError(w, r, "NextPreview: building first group view", "Error loading first group", "error", err)
				return
			}
		} else {
			// Fallback to normal navigation view if no groups/locations configured
			view, err = h.navigationService.GetPlayerNavigationView(r.Context(), team)
			if err != nil {
				if errors.Is(err, services.ErrAllLocationsVisited) && !view.MustCheckOut {
					h.redirect(w, r, "/complete")
					return
				}
				h.handleError(w, r, "NextPreview: getting navigation view", "Error loading navigation", "error", err)
				return
			}
		}
	}

	nextData := templates.NextParams{
		Team: *team,
		View: view,
	}

	err = templates.Next(nextData).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "NextPreview: rendering template", "Error rendering template", "error", err)
	}
}
