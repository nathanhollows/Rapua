package players

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/services"
)

// FinishSection records a player choosing to end a section.
//
// For a section whose band is a range, this is not a dismissal: reaching the
// band's minimum only offers the button, and this press is what completes the
// section. Completing one can complete an ancestor whose own band is now met,
// so the service recomputes the frontier from the stored facts rather than
// trusting what the page was showing.
func (h *PlayerHandler) FinishSection(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	slug := chi.URLParam(r, "slug")
	objective, err := h.checkInService.GetObjectiveByQuestIDAndSlug(r.Context(), team.QuestID, slug)
	if err != nil {
		h.logger.WarnContext(r.Context(), "FinishSection: objective not found",
			"team", team.Code, "objective", slug)
		h.redirect(w, r, "/objectives")
		return
	}

	if _, err := h.navigationService.FinishSection(r.Context(), team, objective.ID); err != nil {
		// A section the run cannot finish is an expected outcome of a stale
		// page, not a fault: the player is simply sent back to a fresh list.
		if errors.Is(err, services.ErrSectionNotFinishable) {
			h.logger.WarnContext(r.Context(), "FinishSection: not finishable",
				"team", team.Code, "objective", slug)
			h.redirect(w, r, "/objectives")
			return
		}
		h.handleError(w, r, "FinishSection: recording finish",
			"Error finishing section", "Could not save progress", err)
		return
	}

	h.redirect(w, r, "/objectives")
}
