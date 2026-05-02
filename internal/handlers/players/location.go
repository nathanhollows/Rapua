package players

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/players"
	"github.com/nathanhollows/Rapua/v7/models"
)

// CheckInView shows the page for a specific location.
func (h *PlayerHandler) CheckInView(w http.ResponseWriter, r *http.Request) {
	// If the user is in preview mode, show the preview
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.checkInPreview(w, r)
		return
	}

	slug := chi.URLParam(r, "slug")

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.logger.Error("loading team", "error", err.Error())
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	var index int
	err = h.teamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.Error("loading team relations", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// Get the index of the location in the team's scans
	index = -1
	for i, scan := range team.CheckIns {
		if scan.Location.Slug == slug {
			index = i
			break
		}
	}

	if index == -1 {
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	contentBlocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
		r.Context(),
		team.CheckIns[index].Location.ID,
		team.Code,
		blocks.ContextLocationContent,
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"CheckInView: getting blocks",
			"Error loading blocks",
			"error",
			err,
			"team",
			team.Code,
			"location",
			slug,
		)
		return
	}

	// Filter blocks by visibility conditions
	resolver := services.NewPlayerVarResolver(team, team.VarStates)
	visibleBlocks := make(blocks.Blocks, 0, len(contentBlocks))
	visibleStates := make(map[string]blocks.PlayerState, len(blockStates))
	for _, b := range contentBlocks {
		if game.EvaluateWhen(b.GetWhen(), resolver) {
			visibleBlocks = append(visibleBlocks, b)
			if s, ok := blockStates[b.GetID()]; ok {
				visibleStates[b.GetID()] = s
			}
		}
	}

	// Get navigation view to determine current group settings
	view, err := h.navigationService.GetPlayerNavigationView(r.Context(), team)
	if err != nil {
		// Continue without view if it fails
		h.logger.Error("getting navigation view", "error", err.Error())
	}

	data := templates.CheckInViewData{
		Settings: team.Instance.Settings,
		Scan:     team.CheckIns[index],
		Blocks:   visibleBlocks,
		States:   visibleStates,
		View:     view,
	}

	c := templates.CheckInView(data)
	err = templates.Layout(c, team.CheckIns[index].Location.Name, team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering checkin view", "error", err.Error())
	}
}

// checkInPreview shows a player preview of the given location.
func (h *PlayerHandler) checkInPreview(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "LocationPreview: getting team", "Error getting team", "error", err)
		return
	}

	err = h.teamService.LoadRelation(r.Context(), team, "Instance")
	if err != nil {
		h.handleError(w, r, "LocationPreview: loading instance", "Error loading instance", "error", err)
		return
	}

	var location models.Location
	found := false
	for _, loc := range team.Instance.Locations {
		if loc.Slug == slug {
			location = loc
			found = true
			break
		}
	}
	if !found {
		h.handleError(w, r, "LocationPreview: finding location", "Location not found", "error", "Location not found")
		return
	}

	scan := models.CheckIn{
		Location: location,
	}

	contentBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		location.ID,
		blocks.ContextLocationContent,
	)
	if err != nil {
		h.handleError(w, r, "LocationPreview: getting blocks", "Error getting blocks", "error", err)
		return
	}

	blockStates := make(map[string]blocks.PlayerState, len(contentBlocks))
	for _, block := range contentBlocks {
		blockStates[block.GetID()], err = h.blockService.NewMockBlockState(r.Context(), block.GetID(), "")
		if err != nil {
			h.handleError(w, r, "LocationPreview: creating block state", "Error creating block state", "error", err)
			return
		}
	}

	data := templates.CheckInViewData{
		Settings: team.Instance.Settings,
		Scan:     scan,
		Blocks:   contentBlocks,
		States:   blockStates,
		View:     nil,
	}

	err = templates.CheckInView(data).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("LocationPreview: rendering template", "error", err)
	}
}
