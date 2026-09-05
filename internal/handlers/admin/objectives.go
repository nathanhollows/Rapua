package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/middlewares"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// ObjectiveNew creates the objective with a default title and redirects to edit.
func (h *Handler) ObjectiveNew(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// This creates a row on a GET, so the route middleware's method check does
	// not cover it and the lock script cannot disable a link.
	if !middlewares.QuestEditable(&user.CurrentQuest, r) {
		h.handleError(w, r, "ObjectiveNew: quest is running", middlewares.QuestRunningMessage)
		return
	}

	// Where the objective goes is decided before it exists: creating it loose
	// and moving it after would leave a moment with two roots.
	parentID := r.URL.Query().Get("parentId")
	if parentID == "" {
		root, rootErr := h.objectiveService.FindRoot(r.Context(), user.CurrentQuestID)
		if rootErr != nil {
			h.handleError(w, r, "ObjectiveNew: finding root objective",
				"Error creating objective", "error", rootErr)
			return
		}
		parentID = root.ID
	}

	objective, err := h.objectiveService.CreateObjective(
		r.Context(), user.CurrentQuestID, parentID, "New Objective",
	)
	if err != nil {
		h.handleError(w, r, "ObjectiveNew: creating objective", "Error creating objective", "error", err)
		return
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

	children, err := h.objectiveService.FindChildren(r.Context(), user.CurrentQuestID, objective.ID)
	if err != nil {
		h.handleError(w, r, "ObjectiveEdit: finding children", "Error finding objective", "error", err)
		return
	}

	allObjectives, err := h.objectiveService.FindByQuestID(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.handleError(w, r, "ObjectiveEdit: finding quest objectives", "Error finding objective", "error", err)
		return
	}

	data := templates.EditObjectiveData{
		Settings:       user.CurrentQuest.Settings,
		Objective:      *objective,
		ProofBlocks:    proofBlocks,
		RevealBlocks:   revealBlocks,
		ChildCount:     len(children),
		DependsOptions: dependsOptions(allObjectives, objective.ID),
	}

	c := templates.LockedEditor(user.CurrentQuest, templates.EditObjective(data))
	err = templates.Layout(c, *user, "Quest", "Edit Objective").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "ObjectiveEdit: rendering template", "Error rendering template", "error", err)
	}
}

// boolPtr names a visibility decision. ObjectiveUpdateData.Draft is a pointer
// so a form that does not offer the control leaves the state alone.
func boolPtr(b bool) *bool { return &b }

// publishedFormValue is the "published" checkbox's on-value, both where the
// full edit form submits it and where the tree's eye toggle does.
const publishedFormValue = "true"

func strPtr(s string) *string { return &s }

func intPtr(v int) *int { return &v }

// parseBandBound returns nil for a blank field: omitted differs from an
// explicit 0 (see game.FillBand).
func parseBandBound(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil //nolint:nilnil // nil means "omitted," not an error
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// parseDepends splits on comma only: a "not " prefix must stay inside its
// entry (see game.ParseDependsName), so whitespace can't also be a separator.
func parseDepends(raw string) []string {
	var entries []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			entries = append(entries, part)
		}
	}
	return entries
}

// dependsOptions is sorted so the picker doesn't reshuffle between renders.
// The objective's own descendants are excluded: naming one is a gate that can
// never open.
func dependsOptions(objectives []models.Objective, excludeID string) []string {
	parentOf := make(map[string]string, len(objectives))
	for _, obj := range objectives {
		parentOf[obj.ID] = obj.ParentID
	}
	isDescendant := func(id string) bool {
		for p := id; p != ""; p = parentOf[p] {
			if p == excludeID {
				return true
			}
		}
		return false
	}

	seen := make(map[string]bool)
	var opts []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		opts = append(opts, name)
	}
	for _, obj := range objectives {
		if obj.ID != excludeID && !isDescendant(obj.ID) {
			add(obj.Slug)
		}
		for _, s := range obj.ProofSets {
			add(s)
		}
		for _, s := range obj.RevealSets {
			add(s)
		}
	}
	slices.Sort(opts)
	return opts
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

	// The checkbox is absent from the form when unticked, so its absence is the
	// instruction to park rather than a form that said nothing.
	published := r.FormValue("published") == publishedFormValue

	minBound, err := parseBandBound(r.FormValue("children_min"))
	if err != nil {
		h.handleError(w, r, "ObjectiveEditPost: parsing children_min", "Invalid completion band", "error", err)
		return
	}
	maxBound, err := parseBandBound(r.FormValue("children_max"))
	if err != nil {
		h.handleError(w, r, "ObjectiveEditPost: parsing children_max", "Invalid completion band", "error", err)
		return
	}
	maxNextBound, err := parseBandBound(r.FormValue("max_next"))
	if err != nil {
		h.handleError(w, r, "ObjectiveEditPost: parsing max_next", "Invalid randomised window", "error", err)
		return
	}
	maxNext := 0
	if maxNextBound != nil {
		maxNext = *maxNextBound
	}

	data := services.ObjectiveUpdateData{
		Title:       r.FormValue("title"),
		Draft:       boolPtr(!published),
		Routing:     strPtr(r.FormValue("routing")),
		MaxNext:     intPtr(maxNext),
		ChildrenMin: minBound,
		ChildrenMax: maxBound,
		FinishLabel: strPtr(r.FormValue("finish_label")),
		Depends:     parseDepends(r.FormValue("depends")),
		Color:       strPtr(r.FormValue("color")),
	}

	err = h.objectiveService.UpdateObjective(r.Context(), objective, data)
	if errors.Is(err, services.ErrParkingBreaksBand) ||
		errors.Is(err, services.ErrCannotDraftRoot) ||
		errors.Is(err, services.ErrInvalidRouting) ||
		errors.Is(err, services.ErrInvalidBand) ||
		errors.Is(err, services.ErrDependsOnDescendant) {
		h.handleError(w, r, "ObjectiveEditPost: refused settings change", err.Error(), "error", err)
		return
	}
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

// ObjectiveSettingsPost touches only the fields the tree's popover submits:
// the pointer update data leaves everything it does not name alone.
func (h *Handler) ObjectiveSettingsPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "ObjectiveSettingsPost: parsing form", "Could not update settings", "error", err)
		return
	}

	user := h.UserFromContext(r.Context())
	objectiveSlug := chi.URLParam(r, "slug")

	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, objectiveSlug)
	if err != nil {
		h.handleError(w, r, "ObjectiveSettingsPost: finding objective", "Objective not found", "error", err)
		return
	}

	minBound, err := parseBandBound(r.FormValue("children_min"))
	if err != nil {
		h.handleError(w, r, "ObjectiveSettingsPost: parsing children_min", "Invalid completion band", "error", err)
		return
	}
	maxBound, err := parseBandBound(r.FormValue("children_max"))
	if err != nil {
		h.handleError(w, r, "ObjectiveSettingsPost: parsing children_max", "Invalid completion band", "error", err)
		return
	}
	maxNextBound, err := parseBandBound(r.FormValue("max_next"))
	if err != nil {
		h.handleError(w, r, "ObjectiveSettingsPost: parsing max_next", "Invalid randomised window", "error", err)
		return
	}
	maxNext := 0
	if maxNextBound != nil {
		maxNext = *maxNextBound
	}

	data := services.ObjectiveUpdateData{
		Routing:     strPtr(r.FormValue("routing")),
		MaxNext:     intPtr(maxNext),
		ChildrenMin: minBound,
		ChildrenMax: maxBound,
		Color:       strPtr(r.FormValue("color")),
	}

	err = h.objectiveService.UpdateObjective(r.Context(), objective, data)
	if errors.Is(err, services.ErrInvalidRouting) || errors.Is(err, services.ErrInvalidBand) {
		h.handleError(w, r, "ObjectiveSettingsPost: refused settings change", err.Error(), "error", err)
		return
	}
	if err != nil {
		h.handleError(w, r, "ObjectiveSettingsPost: updating objective", "Error updating settings", "error", err)
		return
	}

	h.handleSuccess(w, r, "Settings updated")
}

// ObjectiveDraftTogglePost touches Draft only: the rest of
// ObjectiveUpdateData is left at its zero/nil "unchanged" value.
func (h *Handler) ObjectiveDraftTogglePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "ObjectiveDraftTogglePost: parsing form", "Could not update visibility", "error", err)
		return
	}

	user := h.UserFromContext(r.Context())
	objectiveSlug := chi.URLParam(r, "slug")

	objective, err := h.objectiveService.GetByQuestIDAndSlug(r.Context(), user.CurrentQuestID, objectiveSlug)
	if err != nil {
		h.handleError(w, r, "ObjectiveDraftTogglePost: finding objective", "Objective not found", "error", err)
		return
	}

	published := r.FormValue("published") == publishedFormValue
	err = h.objectiveService.UpdateObjective(r.Context(), objective, services.ObjectiveUpdateData{
		Draft: boolPtr(!published),
	})
	if errors.Is(err, services.ErrParkingBreaksBand) || errors.Is(err, services.ErrCannotDraftRoot) {
		h.handleError(w, r, "ObjectiveDraftTogglePost: refused visibility change", err.Error(), "error", err)
		return
	}
	if err != nil {
		h.handleError(w, r, "ObjectiveDraftTogglePost: updating objective", "Error updating visibility", "error", err)
		return
	}

	h.handleSuccess(w, r, "Visibility updated")
}

// ObjectiveReposition always responds with a flash, never a redirect: the
// tree re-fetches regardless of outcome, refused moves included.
func (h *Handler) ObjectiveReposition(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "ObjectiveReposition: parsing form", "Could not move objective", "error", err)
		return
	}

	user := h.UserFromContext(r.Context())

	objectiveID := r.FormValue("objective_id")
	parentID := r.FormValue("parent_id")
	position, err := strconv.Atoi(r.FormValue("position"))
	if err != nil {
		h.handleError(w, r, "ObjectiveReposition: parsing position", "Could not move objective", "error", err)
		return
	}

	err = h.objectiveService.Reposition(r.Context(), user.CurrentQuestID, objectiveID, parentID, position)
	if err != nil {
		h.handleError(w, r, "ObjectiveReposition: moving objective", "Could not move objective", "error", err)
		return
	}

	h.handleSuccess(w, r, "Objective moved")
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
