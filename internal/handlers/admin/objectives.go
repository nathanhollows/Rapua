package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
)

// ObjectiveNew creates the objective with a default title and redirects to edit.
func (h *Handler) ObjectiveNew(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	objective, err := h.objectiveService.CreateObjective(r.Context(), user.CurrentQuestID, "New Objective")
	if err != nil {
		h.handleError(w, r, "ObjectiveNew: creating objective", "Error creating objective", "error", err)
		return
	}

	groupID := r.URL.Query().Get("groupId")
	afterObjectiveID := r.URL.Query().Get("afterObjectiveId")
	beforeObjectiveID := r.URL.Query().Get("beforeObjectiveId")

	if groupID != "" {
		if err = h.gameStructureService.InsertObjectiveIntoGroup(
			r.Context(), user.CurrentQuestID, objective.ID, groupID, afterObjectiveID, beforeObjectiveID,
		); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"ObjectiveNew: inserting objective into group",
				"error",
				err,
				"objective_id",
				objective.ID,
			)
		}
	} else {
		if err = h.addObjectiveToRootGroup(r.Context(), user.CurrentQuestID, objective.ID); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"ObjectiveNew: adding objective to root group",
				"error",
				err,
				"objective_id",
				objective.ID,
			)
		}
	}

	editPath := "/admin/objective/" + objective.Slug
	if r.Header.Get("Hx-Request") == htmxHeaderTrue {
		w.Header().Set("Hx-Location", editPath)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, editPath, http.StatusFound)
}

func (h *Handler) ObjectiveEdit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "ObjectiveEdit: parsing form", "Error parsing form", "error", err)
		return
	}

	slug := chi.URLParam(r, "slug")
	user := h.UserFromContext(r.Context())

	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.WarnContext(r.Context(),
				"ObjectiveEdit: objective not found",
				"quest_id",
				user.CurrentQuestID,
				"objective_slug",
				slug,
			)
			h.redirect(w, r, "/admin/quest")
			return
		}
		h.handleError(w, r, "ObjectiveEdit: finding objective", "Error finding objective", "error", err)
		return
	}

	proofBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		objective.ID,
		blocks.ContextObjectiveProof,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"ObjectiveEdit: getting proof blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"objective_id",
			objective.ID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	revealBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		objective.ID,
		blocks.ContextObjectiveReveal,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"ObjectiveEdit: getting reveal blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"objective_id",
			objective.ID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	data := templates.EditObjectiveData{
		Settings:     user.CurrentQuest.Settings,
		Objective:    *objective,
		ProofBlocks:  proofBlocks,
		RevealBlocks: revealBlocks,
	}

	c := templates.EditObjective(data)
	err = templates.Layout(c, *user, "Quest", "Edit Objective").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "ObjectiveEdit: rendering template", "Error rendering template", "error", err)
	}
}

func (h *Handler) ObjectiveEditPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "ObjectiveEditPost: parsing form", "Error parsing form", "error", err)
		return
	}

	user := h.UserFromContext(r.Context())
	objectiveSlug := chi.URLParam(r, "slug")

	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, objectiveSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.handleError(w, r, "ObjectiveEditPost: objective not found", "Objective not found",
				"quest_id", user.CurrentQuestID, "objective_slug", objectiveSlug)
			return
		}
		h.handleError(w, r, "ObjectiveEditPost: finding objective", "Error finding objective", "error", err)
		return
	}

	data := services.ObjectiveUpdateData{
		Title: r.FormValue("title"),
	}

	err = h.objectiveService.UpdateObjective(r.Context(), objective, data)
	if err != nil {
		h.handleError(w, r, "ObjectiveEditPost: updating objective", "Error updating objective", "error", err)
		return
	}

	if objective.Slug != objectiveSlug {
		h.redirect(w, r, "/admin/objective/"+objective.Slug)
		return
	}

	h.handleSuccess(w, r, "Objective updated")
}

func (h *Handler) ObjectiveDelete(w http.ResponseWriter, r *http.Request) {
	objectiveSlug := chi.URLParam(r, "slug")

	user := h.UserFromContext(r.Context())

	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, objectiveSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.handleError(w, r, "ObjectiveDelete: objective not found", "Objective not found",
				"quest_id", user.CurrentQuestID, "objective_slug", objectiveSlug)
			return
		}
		h.handleError(w, r, "ObjectiveDelete: finding objective", "Error finding objective", "error", err)
		return
	}

	if err = h.deleteService.DeleteObjective(r.Context(), objective.ID); err != nil {
		h.handleError(w, r, "ObjectiveDelete: deleting objective", "Error deleting objective", "error", err)
		return
	}

	h.redirect(w, r, "/admin/quest")
}

func (h *Handler) addObjectiveToRootGroup(ctx context.Context, questID, objectiveID string) error {
	instance, err := h.questService.GetByID(ctx, questID)
	if err != nil {
		return fmt.Errorf("loading instance: %w", err)
	}

	instance.GameStructure.ObjectiveIDs = append(instance.GameStructure.ObjectiveIDs, objectiveID)

	if err = h.gameStructureService.Save(ctx, questID, &instance.GameStructure); err != nil {
		return fmt.Errorf("saving structure: %w", err)
	}

	return nil
}
