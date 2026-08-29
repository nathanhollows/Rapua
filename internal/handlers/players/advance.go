package players

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/models"
)

func (h *PlayerHandler) AdvanceGroup(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	h.advanceObjectiveGroup(w, r, team)
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
