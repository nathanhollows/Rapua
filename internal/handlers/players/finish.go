package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
)

func (h *PlayerHandler) Finish(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	// Skip redirect logic in preview mode
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		locations, err := h.navigationService.GetNextLocations(r.Context(), team)
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

	// Get blocks for the finish page
	pageBlocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
		r.Context(),
		team.InstanceID,
		team.Code,
		blocks.ContextFinish,
	)
	if err != nil {
		h.logger.Error("getting finish blocks", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}
	// If the user is in preview mode, only render the template, not the full layout.
	template := templates.Start(*team, pageBlocks, blockStates)
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		template = templates.Layout(template, "Start", team.Messages)
	}

	// data["notifications"], _ = h.NotificationService.GetNotifications(r.Context(), team.Code)
	c := templates.Finish(*team, pageBlocks, blockStates)
	err = templates.Layout(c, "Finish", team.Messages).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Next: rendering template", "Error rendering template", "Could not render template", err)
	}
}
