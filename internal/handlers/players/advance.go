package players

import (
	"net/http"
)

// AdvanceGroup allows a team to manually skip the current group and advance to the next one.
// This delegates validation to the navigation service via CanAdvanceEarly flag.
func (h *PlayerHandler) AdvanceGroup(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

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
	err = h.teamService.Update(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "AdvanceGroup: updating team", "Error advancing group", "Could not save progress", err)
		return
	}

	// Redirect back to /next to show the new current group
	h.redirect(w, r, "/next")
}
