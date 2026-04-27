package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/players"
)

func (h *PlayerHandler) Complete(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	// Skip redirect logic in preview mode
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		// Use the same navigation view as /next so both handlers agree on
		// remaining locations (including when-clause filtering).
		view, navErr := h.navigationService.GetPlayerNavigationView(r.Context(), team)
		if navErr != nil {
			if !errors.Is(navErr, services.ErrAllLocationsVisited) {
				h.handleError(
					w,
					r,
					"Complete: getting navigation view",
					"Error getting navigation view",
					"Could not load data",
					navErr,
				)
				return
			}
			// ErrAllLocationsVisited — fall through to show complete page
		} else if len(view.NextLocations) > 0 {
			h.redirect(w, r, "/next")
			return
		}
	}

	// Get blocks for the complete page
	var pageBlocks []blocks.Block
	var blockStates map[string]blocks.PlayerState
	// Same preview guard as start.go: no DB row for the preview team.
	teamCode := team.Code
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		teamCode = ""
	}
	pageBlocks, blockStates, err = h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
		r.Context(),
		team.InstanceID,
		teamCode,
		blocks.ContextFinish,
	)
	if err != nil {
		h.logger.Error("getting 'complete' page blocks", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// If the user is in preview mode, only render the template, not the full layout.
	template := templates.Complete(*team, pageBlocks, blockStates)
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		template = templates.Layout(template, "Complete", team.Messages)
	}

	err = template.Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering 'complete' page", "error", err.Error())
	}
}
