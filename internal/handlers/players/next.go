package players

import (
	"errors"
	"maps"
	"net/http"

	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/players"
	"github.com/nathanhollows/Rapua/v4/models"
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

	locations, err := h.navigationService.GetNextLocations(r.Context(), team)
	if err != nil {
		if errors.Is(err, services.ErrAllLocationsVisited) && team.MustCheckOut == "" {
			h.redirect(w, r, "/finish")
			return
		}
		h.handleError(w, r, "Next: getting next locations", "Error getting next locations", "Could not load data", err)
		return
	}

	// Fetch navigation clue blocks for all next locations
	allBlocks := make([]blocks.Block, 0)
	allStates := make(map[string]blocks.PlayerState)

	if team.Instance.Settings.NavigationDisplayMode == models.NavigationDisplayCustom {
		for _, location := range locations {
			locationBlocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
				r.Context(),
				location.ID,
				team.Code,
				blocks.ContextLocationClues,
			)
			if err != nil {
				h.handleError(
					w,
					r,
					"Next: getting navigation blocks",
					"Error loading navigation clues",
					"Could not load navigation clues",
					err,
				)
				return
			}
			allBlocks = append(allBlocks, locationBlocks...)
			maps.Copy(allStates, blockStates)
		}
	}

	nextData := templates.NextParams{
		Team:      *team,
		Settings:  team.Instance.Settings,
		Locations: locations,
		Blocks:    allBlocks,
		States:    allStates,
	}

	template := templates.Next(nextData)
	template = templates.Layout(template, "Next stops", team.Messages)
	err = template.Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Next: rendering template", "Error rendering template", "Could not render template", err)
	}
}

// nextPreview shows a preview of the navigation page for admins.
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

	locations, err := h.navigationService.GetNextLocations(r.Context(), team)
	if err != nil {
		if errors.Is(err, services.ErrAllLocationsVisited) && team.MustCheckOut == "" {
			h.redirect(w, r, "/finish")
			return
		}
		h.handleError(w, r, "Next: getting next locations", "Error getting next locations", "Could not load data", err)
		return
	}

	// Fetch navigation clue blocks for all locations
	allBlocks := make([]blocks.Block, 0)
	allStates := make(map[string]blocks.PlayerState)

	if team.Instance.Settings.NavigationDisplayMode == models.NavigationDisplayCustom {
		for _, location := range locations {
			locationBlocks, err := h.blockService.FindByOwnerIDAndContext(
				r.Context(),
				location.ID,
				blocks.ContextLocationClues,
			)
			if err != nil {
				h.handleError(w, r, "NextPreview: getting navigation blocks", "Error getting blocks", "error", err)
				return
			}

			// Create mock states for preview
			for _, block := range locationBlocks {
				mockState, err := h.blockService.NewMockBlockState(r.Context(), block.GetID(), "")
				if err != nil {
					h.handleError(w, r, "NextPreview: creating block state", "Error creating block state", "error", err)
					return
				}
				allStates[block.GetID()] = mockState
			}

			allBlocks = append(allBlocks, locationBlocks...)
		}
	}

	nextData := templates.NextParams{
		Team:      *team,
		Settings:  team.Instance.Settings,
		Locations: locations,
		Blocks:    allBlocks,
		States:    allStates,
	}

	err = templates.Next(nextData).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "NextPreview: rendering template", "Error rendering template", "error", err)
	}
}
