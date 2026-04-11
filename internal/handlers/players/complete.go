package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/players"
	"github.com/nathanhollows/Rapua/v7/models"
)

func (h *PlayerHandler) Complete(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	// Skip redirect logic in preview mode
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		var locations []models.Location
		locations, err = h.navigationService.GetNextLocations(r.Context(), team)
		if err != nil {
			if !errors.Is(err, services.ErrAllLocationsVisited) {
				h.handleError(
					w,
					r,
					"Next: getting next locations",
					"Error getting next locations",
					"Could not load data",
					err,
				)
				return
			}
		}
		if len(locations) > 0 {
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
