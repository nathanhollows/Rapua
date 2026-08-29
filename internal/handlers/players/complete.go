package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

func (h *PlayerHandler) Complete(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	// Skip redirect logic in preview mode
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		if !isObjectiveBuiltQuest(team.Quest.GameStructure) {
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
				// ErrAllLocationsVisited: fall through to show complete page.
			} else if len(view.NextLocations) > 0 {
				h.redirect(w, r, "/next")
				return
			}
		} else {
			// Use the same objective view as /objectives so both handlers agree.
			view, objErr := h.navigationService.GetPlayerObjectiveView(r.Context(), team)
			if objErr != nil {
				if !errors.Is(objErr, services.ErrAllObjectivesVisited) {
					h.handleError(
						w,
						r,
						"Complete: getting objective view",
						"Error getting objective view",
						"Could not load data",
						objErr,
					)
					return
				}
				// ErrAllObjectivesVisited: fall through to show complete page.
			} else if len(view.NextObjectives) > 0 {
				h.redirect(w, r, "/objectives")
				return
			}
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
	pageBlocks, blockStates, err = h.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		r.Context(),
		team.QuestID,
		teamCode,
		team.QuestID,
		blocks.ContextFinish,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "getting 'complete' page blocks", "error", err.Error())
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
		h.logger.ErrorContext(r.Context(), "rendering 'complete' page", "error", err.Error())
	}
}
