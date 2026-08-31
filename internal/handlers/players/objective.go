package players

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
)

// ObjectiveView shows the page for a specific objective: proof content while
// unproven, reveal content once proof completes.
func (h *PlayerHandler) ObjectiveView(w http.ResponseWriter, r *http.Request) {
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.objectivePreview(w, r)
		return
	}

	slug := chi.URLParam(r, "slug")

	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "loading team", "error", err.Error())
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "loading team relations", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	objective, err := h.checkInService.GetObjectiveByQuestIDAndSlug(r.Context(), team.QuestID, slug)
	if err != nil {
		// A bad or stale slug is an expected outcome, unlike the errors below
		// (real faults): this warns and redirects rather than rendering the
		// bare error toast handleError produces, since full-page GET
		// navigation needs a full page back, not a fragment with no layout.
		// Any other error (DB outage, etc.) must not be downgraded to "not
		// found" silently, so it still goes through handleError.
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.WarnContext(r.Context(), "ObjectiveView: objective not found", "team", team.Code, "objective", slug)
			http.Redirect(w, r, "/objectives", http.StatusFound)
			return
		}
		h.handleError(
			w,
			r,
			"ObjectiveView: finding objective",
			"Something went wrong",
			"error",
			err,
			"team",
			team.Code,
			"objective",
			slug,
		)
		return
	}

	if h.redirectIfUnreachable(w, r, team, objective) {
		return
	}

	pending, err := h.checkInService.IsObjectiveContextPending(r.Context(), team, objective.ID, blocks.ContextObjectiveProof)
	if err != nil {
		h.handleError(
			w,
			r,
			"ObjectiveView: checking proof completion",
			"Something went wrong",
			"error",
			err,
			"team",
			team.Code,
			"objective",
			slug,
		)
		return
	}

	zone := blocks.ContextObjectiveProof
	if !pending {
		// Content-only zones have nothing a player can POST to, so completion
		// (sets, logging) for them is only ever triggered from here.
		if err := h.checkInService.CompleteObjectiveContext(r.Context(), team, objective.ID, blocks.ContextObjectiveProof); err != nil {
			h.logger.ErrorContext(r.Context(), "ObjectiveView: completing proof context", "error", err.Error())
		}
		if err := h.checkInService.CompleteObjectiveContext(r.Context(), team, objective.ID, blocks.ContextObjectiveReveal); err != nil {
			h.logger.ErrorContext(r.Context(), "ObjectiveView: completing reveal context", "error", err.Error())
		}
		zone = blocks.ContextObjectiveReveal
	}

	contentBlocks, blockStates, err := h.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		r.Context(),
		objective.ID,
		team.Code,
		team.QuestID,
		zone,
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"ObjectiveView: getting blocks",
			"Error loading blocks",
			"error",
			err,
			"team",
			team.Code,
			"objective",
			slug,
		)
		return
	}

	data := templates.ObjectiveViewData{
		Settings: team.Quest.Settings,
		Zone:     zone,
		Blocks:   contentBlocks,
		States:   blockStates,
	}

	c := templates.ObjectiveView(data)
	err = templates.Layout(c, objective.Title, team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering objective view", "error", err.Error())
	}
}

// redirectIfUnreachable sends a player who reached a gated objective's URL
// back to the objectives list, and reports whether it did. Rendering it would
// spoil content the run has not unlocked; CheckInService gates the completion
// side, which is the half that would otherwise corrupt run state.
func (h *PlayerHandler) redirectIfUnreachable(
	w http.ResponseWriter, r *http.Request, team *models.Run, objective *models.Objective,
) bool {
	reachable, err := h.checkInService.ObjectiveIsReachable(r.Context(), team, objective)
	if err != nil {
		h.handleError(
			w, r, "ObjectiveView: checking reachability", "Something went wrong",
			"error", err, "team", team.Code, "objective", objective.Slug,
		)
		return true
	}
	if reachable {
		return false
	}

	// Same treatment as a stale slug: an unmet depends is an expected outcome
	// of guessing a URL, not a fault worth an error toast.
	h.logger.WarnContext(r.Context(), "ObjectiveView: objective not yet reachable",
		"team", team.Code, "objective", objective.Slug)
	http.Redirect(w, r, "/objectives", http.StatusFound)
	return true
}

// objectivePreview shows a player preview of the given objective's proof or
// reveal zone. Unlike the normal path, it never calls
// IsObjectiveContextPending/CompleteObjectiveContext: there's no real
// completion state for a preview run, so the zone is chosen directly from the
// ?zone= query param (defaulting to proof) and blocks are rendered with
// mocked, unpersisted states, mirroring checkInPreview.
func (h *PlayerHandler) objectivePreview(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "ObjectivePreview: getting team", "Error getting team", "error", err)
		return
	}

	objective, err := h.checkInService.GetObjectiveByQuestIDAndSlug(r.Context(), team.QuestID, slug)
	if err != nil {
		h.handleError(w, r, "ObjectivePreview: finding objective", "Objective not found", "error", err)
		return
	}

	zone := blocks.ContextObjectiveProof
	if r.URL.Query().Get("zone") == "reveal" {
		zone = blocks.ContextObjectiveReveal
	}

	contentBlocks, err := h.blockService.FindByOwnerIDAndContext(r.Context(), objective.ID, zone)
	if err != nil {
		h.handleError(w, r, "ObjectivePreview: getting blocks", "Error getting blocks", "error", err)
		return
	}

	blockStates := make(map[string]blocks.PlayerState, len(contentBlocks))
	for _, block := range contentBlocks {
		blockStates[block.GetID()], err = h.blockService.NewMockBlockState(r.Context(), block.GetID(), "", "")
		if err != nil {
			h.handleError(w, r, "ObjectivePreview: creating block state", "Error creating block state", "error", err)
			return
		}
	}

	data := templates.ObjectiveViewData{
		Settings: team.Quest.Settings,
		Zone:     zone,
		Blocks:   contentBlocks,
		States:   blockStates,
	}

	err = templates.ObjectiveView(data).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ObjectivePreview: rendering template", "error", err)
	}
}
