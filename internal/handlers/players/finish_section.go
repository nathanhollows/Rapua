package players

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/services"
)

// Every exit is a plain 303, never h.redirect: the press is a boosted form, and
// an Hx-Redirect would reload the whole document.
//
// For a section whose band is a range, this is not a dismissal: reaching the
// band's minimum only offers the button, and this press is what completes the
// section. Completing one can complete an ancestor whose own band is now met,
// so the service recomputes the frontier from the stored facts rather than
// trusting what the page was showing.
func (h *PlayerHandler) FinishSection(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		http.Redirect(w, r, "/play", http.StatusSeeOther)
		return
	}

	slug := chi.URLParam(r, "slug")
	objective, err := h.checkInService.GetObjectiveByQuestIDAndSlug(r.Context(), team.QuestID, slug)
	if err != nil {
		h.logger.WarnContext(r.Context(), "FinishSection: objective not found",
			"team", team.Code, "objective", slug)
		http.Redirect(w, r, "/objectives", http.StatusSeeOther)
		return
	}

	if _, err := h.navigationService.FinishSection(r.Context(), team, objective.ID); err != nil {
		// A section the run cannot finish is an expected outcome of a stale
		// page, not a fault: the player is simply sent back to a fresh list.
		if errors.Is(err, services.ErrSectionNotFinishable) {
			h.logger.WarnContext(r.Context(), "FinishSection: not finishable",
				"team", team.Code, "objective", slug)
			http.Redirect(w, r, "/objectives", http.StatusSeeOther)
			return
		}
		h.handleError(w, r, "FinishSection: recording finish",
			"Error finishing section", "Could not save progress", err)
		return
	}

	http.Redirect(w, r, "/objectives", http.StatusSeeOther)
}
