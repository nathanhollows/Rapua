package players

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
	"github.com/nathanhollows/Rapua/v6/models"
)

// CheckInView shows the page for a specific location.
func (h *PlayerHandler) CheckInView(w http.ResponseWriter, r *http.Request) {
	// If the user is in preview mode, show the preview
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.checkInPreview(w, r)
		return
	}

	locationCode := chi.URLParam(r, "id")

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
		if scan.Location.MarkerID == locationCode {
			index = i
			break
		}
	}

	if index == -1 {
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	blocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
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
			locationCode,
		)
		return
	}

	data := templates.CheckInViewData{
		Settings: team.Instance.Settings,
		Scan:     team.CheckIns[index],
		Blocks:   blocks,
		States:   blockStates,
	}

	c := templates.CheckInView(data)
	err = templates.Layout(c, team.CheckIns[index].Location.Name, team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering checkin view", "error", err.Error())
	}
}

// checkInPreview shows a player preview of the given location.
func (h *PlayerHandler) checkInPreview(w http.ResponseWriter, r *http.Request) {
	locationCode := chi.URLParam(r, "id")

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
	for _, loc := range team.Instance.Locations {
		if loc.MarkerID == locationCode {
			location = loc
			break
		}
	}
	if location.MarkerID == "" {
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
	}

	err = templates.CheckInView(data).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("LocationPreview: rendering template", "error", err)
	}
}
