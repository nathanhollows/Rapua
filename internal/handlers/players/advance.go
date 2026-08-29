package players

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/models"
)

// AdvanceGroup allows a team to manually skip the current group and advance to
// the next one. Location and Objective quests validate through different
// navigation views (CanAdvanceEarly means something different for each), so
// this dispatches to the matching path rather than always using the
// location-only view: a location-built quest always has empty ObjectiveIDs
// (and vice versa), so calling the wrong path would silently no-op.
func (h *PlayerHandler) AdvanceGroup(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if isObjectiveBuiltQuest(team.Quest.GameStructure) {
		h.advanceObjectiveGroup(w, r, team)
		return
	}
	h.advanceLocationGroup(w, r, team)
}

func (h *PlayerHandler) advanceLocationGroup(w http.ResponseWriter, r *http.Request, team *models.Run) {
	// Get navigation view which includes CanAdvanceEarly validation
	view, err := h.navigationService.GetPlayerNavigationView(r.Context(), team)
	if err != nil {
		h.handleError(
			w,
			r,
			"AdvanceGroup: getting navigation view",
			"Error loading navigation",
			"Could not load data",
			err,
		)
		return
	}

	// Trust the navigation service's validation
	if !view.CanAdvanceEarly || view.CurrentGroup == nil {
		// Not allowed to advance - redirect back
		h.redirect(w, r, "/next")
		return
	}

	// Add current group to skipped list
	team.SkippedGroupIDs = append(team.SkippedGroupIDs, view.CurrentGroup.ID)

	// Update team in database
	err = h.runService.Update(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "AdvanceGroup: updating team", "Error advancing group", "Could not save progress", err)
		return
	}

	// Redirect back to /next to show the new current group
	h.redirect(w, r, "/next")
}

func (h *PlayerHandler) advanceObjectiveGroup(w http.ResponseWriter, r *http.Request, team *models.Run) {
	view, err := h.navigationService.GetPlayerObjectiveView(r.Context(), team)
	if err != nil {
		h.handleError(
			w,
			r,
			"AdvanceGroup: getting objective view",
			"Error loading navigation",
			"Could not load data",
			err,
		)
		return
	}

	if !view.CanAdvanceEarly || view.CurrentGroup == nil {
		h.redirect(w, r, "/objectives")
		return
	}

	team.SkippedGroupIDs = append(team.SkippedGroupIDs, view.CurrentGroup.ID)

	err = h.runService.Update(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "AdvanceGroup: updating team", "Error advancing group", "Could not save progress", err)
		return
	}

	h.redirect(w, r, "/objectives")
}
