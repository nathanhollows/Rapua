package players

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// Objectives is /objectives' handler: the Objective-built quest counterpart of Next.
func (h *PlayerHandler) Objectives(w http.ResponseWriter, r *http.Request) {
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.objectivesPreview(w, r)
		return
	}

	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	view, err := h.navigationService.GetPlayerObjectiveView(r.Context(), team)
	if err != nil {
		if errors.Is(err, services.ErrAllObjectivesVisited) {
			h.redirect(w, r, "/complete")
			return
		}
		h.handleError(w, r, "Objectives: getting objective view", "Error loading objectives", "Could not load data", err)
		return
	}

	data := templates.ObjectivesParams{
		Run:  *team,
		View: view,
	}

	template := templates.Objectives(data)
	template = templates.Layout(template, "Objectives", team.Messages)
	err = template.Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Objectives: rendering template", "Error rendering template", "Could not render template", err)
	}
}

func (h *PlayerHandler) objectivesPreview(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "ObjectivesPreview: getting team", "Error getting team", "error", err)
		return
	}

	err = h.runService.LoadQuest(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "ObjectivesPreview: loading quest", "Error loading quest", "error", err)
		return
	}

	var view *services.PlayerObjectiveView

	targetObjectiveID := r.URL.Query().Get("objective_id")
	if targetObjectiveID != "" { //nolint:nestif // preview vs. normal navigation requires nested error handling per path
		view, err = h.navigationService.GetPreviewObjectiveView(r.Context(), team, targetObjectiveID)
		if err != nil {
			h.handleError(w, r, "ObjectivesPreview: building objective view", "Error loading objective", "error", err)
			return
		}
	} else {
		// GetFirstVisibleGroup, not SubGroups[0] directly: the first subgroup may be
		// secret (RouteStrategySecret), which must never be what a preview opens on.
		firstGroup := navigation.GetFirstVisibleGroup(&team.Quest.GameStructure)
		if firstGroup != nil && len(firstGroup.ObjectiveIDs) > 0 {
			firstObjectiveID := firstGroup.ObjectiveIDs[0]
			view, err = h.navigationService.GetPreviewObjectiveView(r.Context(), team, firstObjectiveID)
			if err != nil {
				h.handleError(w, r, "ObjectivesPreview: building first group view", "Error loading first group", "error", err)
				return
			}
		} else {
			view, err = h.navigationService.GetPlayerObjectiveView(r.Context(), team)
			if err != nil {
				if errors.Is(err, services.ErrAllObjectivesVisited) {
					h.redirect(w, r, "/complete")
					return
				}
				h.handleError(w, r, "ObjectivesPreview: getting objective view", "Error loading objectives", "error", err)
				return
			}
		}
	}

	data := templates.ObjectivesParams{
		Run:  *team,
		View: view,
	}

	err = templates.Objectives(data).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "ObjectivesPreview: rendering template", "Error rendering template", "error", err)
	}
}

// Journal is /journal's handler: the Objective-built quest counterpart of MyCheckins.
func (h *PlayerHandler) Journal(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil || team == nil {
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.journalPreview(w, r, team)
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		// A blind Referer redirect loops forever when Referer is empty (direct
		// nav, htmx requests): a toast fails safely instead.
		h.handleError(w, r, "Journal: loading team relations", "Error loading journal", "error", err.Error())
		return
	}

	completed, err := h.runService.GetCompletedObjectives(r.Context(), team.QuestID, team.Code)
	if err != nil {
		h.handleError(w, r, "Journal: getting completed objectives", "Error loading journal", "error", err.Error())
		return
	}

	c := templates.Journal(*team, completed)
	err = templates.Layout(c, "My Journal", team.Messages).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering journal", "error", err.Error())
	}
}

func (h *PlayerHandler) journalPreview(w http.ResponseWriter, r *http.Request, team *models.Run) {
	err := h.runService.LoadQuest(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "JournalPreview: loading quest", "Error loading quest", "error", err)
		return
	}

	err = templates.Journal(*team, nil).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "JournalPreview: rendering template", "Error rendering template", "error", err)
	}
}
