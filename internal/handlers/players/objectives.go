package players

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

// Objectives is /objectives' handler: the player's list of available objectives.
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
		h.handleError(
			w,
			r,
			"Objectives: getting objective view",
			"Error loading objectives",
			"Could not load data",
			err,
		)
		return
	}

	// Only the root completing means the run is finished. An empty list can
	// also mean everything left is waiting on something.
	//
	// Drawn in place (see renderComplete), with the address corrected so a reload
	// lands on the finish page.
	if view.Complete {
		if r.Header.Get("Hx-Request") == "true" {
			w.Header().Set("Hx-Replace-Url", "/complete")
		}
		h.renderComplete(w, r, team)
		return
	}

	// After the redirect, so a run that is finished is not recorded as having
	// opened a list it was never shown. A failure here costs a data point, not
	// the page.
	err = h.navigationService.RecordObjectivesListed(r.Context(), team)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "recording objectives listed", "error", err)
	}

	data := templates.ObjectivesParams{
		Run:  *team,
		View: view,
	}

	template := templates.QuestPage(data)
	template = templates.AppLayout(
		template, templates.QuestChrome(*team), "Quest", team.Messages,
	)
	err = template.Render(r.Context(), w)
	if err != nil {
		h.handleError(
			w,
			r,
			"Objectives: rendering template",
			"Error rendering template",
			"Could not render template",
			err,
		)
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
	if targetObjectiveID != "" {
		view, err = h.navigationService.GetPreviewObjectiveView(r.Context(), team, targetObjectiveID)
		if err != nil {
			h.handleError(w, r, "ObjectivesPreview: building objective view", "Error loading objective", "error", err)
			return
		}
	} else {
		view, err = h.navigationService.GetPlayerObjectiveView(r.Context(), team)
		if err != nil {
			h.handleError(w, r, "ObjectivesPreview: getting objective view", "Error loading objectives", "error", err)
			return
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
