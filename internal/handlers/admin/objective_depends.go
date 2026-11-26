package admin

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
)

// ObjectiveDependsEdit opens the editor for the variables an objective waits
// on, offering every variable the quest sets.
// GET /admin/objective/{slug}/depends.
func (h *Handler) ObjectiveDependsEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, chi.URLParam(r, "slug"))
	if err != nil {
		h.handleError(w, r, "ObjectiveDependsEdit: finding objective", "Objective not found", "error", err)
		return
	}
	known, err := h.variableService.QuestVariables(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ObjectiveDependsEdit: listing quest variables", "error", err)
	}
	if err := templates.ObjectiveDependsEditor(*objective, known).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "ObjectiveDependsEdit: rendering editor", "error", err)
	}
}

// ObjectiveDependsPost saves the variables an objective waits on and folds the
// editor back into its button.
// POST /admin/objective/{slug}/depends.
func (h *Handler) ObjectiveDependsPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "ObjectiveDependsPost: parsing form", "Could not save the lock", "error", err)
		return
	}
	user := h.UserFromContext(r.Context())
	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, chi.URLParam(r, "slug"))
	if err != nil {
		h.handleError(w, r, "ObjectiveDependsPost: finding objective", "Objective not found", "error", err)
		return
	}

	depends := r.PostForm["depends"]
	err = h.objectiveService.UpdateObjective(r.Context(), objective, services.ObjectiveUpdateData{Depends: &depends})
	for _, refusal := range []error{services.ErrCannotGateRoot, services.ErrInvalidVarName} {
		if errors.Is(err, refusal) {
			h.handleError(w, r, "ObjectiveDependsPost: refused", refusal.Error(), "error", err)
			return
		}
	}
	if err != nil {
		h.handleError(w, r, "ObjectiveDependsPost: updating objective", "Could not save the lock", "error", err)
		return
	}

	if err := templates.ObjectiveDependsButtonOOB(*objective).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "ObjectiveDependsPost: rendering button", "error", err)
	}
	h.refreshLintPanel(w, r, objective.ID, "")
	h.handleSuccess(w, r, "Lock saved")
}
