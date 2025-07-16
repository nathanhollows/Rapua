package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
)

// Activity displays the activity tracker page.
func (h *AdminHandler) Activity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i := range user.CurrentInstance.Teams {
		if user.CurrentInstance.Teams[i].Code == "" {
			continue // Skip teams without a code
		}
		err := h.TeamService.LoadRelation(r.Context(), &user.CurrentInstance.Teams[i], "Scans")
		if err != nil {
			h.handleError(w, r, "ActivityTeamsOverview: loading team relations", "Error loading team relations", "Could not load data", err)
			return
		}
	}

	c := templates.ActivityTracker(user.CurrentInstance)
	err := templates.Layout(c, *user, "Activity", "Activity").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Activity: rendering template", "error", err)
	}
}

// ActivityTeamsOverview displays the activity tracker page.
func (h *AdminHandler) ActivityTeamsOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i := range user.CurrentInstance.Teams {
		err := h.TeamService.LoadRelation(r.Context(), &user.CurrentInstance.Teams[i], "Scans")
		if err != nil {
			h.handleError(w, r, "ActivityTeamsOverview: loading team relations", "Error loading team relations", "Could not load data", err)
			return
		}
	}

	err := templates.ActivityTeamsTable(user.CurrentInstance.Locations, user.CurrentInstance.Teams).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("ActivityTeamsOverview: rendering template", "error", err)
	}
}

// TeamActivity displays the activity tracker page.
// It accepts HTMX requests to update the team activity.
func (h *AdminHandler) TeamActivity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teamCode := chi.URLParam(r, "teamCode")

	team, err := h.GameplayService.GetTeamByCode(r.Context(), teamCode)
	if err != nil || team.InstanceID != user.CurrentInstanceID {
		h.handleError(w, r, "TeamActivity: getting team", "Error getting team", "Could not load data", err)
		return
	}

	err = h.TeamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "TeamActivity: loading scans", "Error loading data", "Could not load data", err)
		return
	}

	locations, err := h.navigationService.GetNextLocations(r.Context(), team)
	if err != nil {
		if !errors.Is(err, services.ErrAllLocationsVisited) {
			h.handleError(w, r, "TeamActivity: getting next locations", "Error getting next locations", "Could not load data", err)
			return
		}
	}

	notifications, err := h.NotificationService.GetNotifications(r.Context(), team.Code)
	if err != nil {
		h.handleError(w, r, "TeamActivity: getting notifications", "Error getting notifications", "Could not load data", err)
		return
	}

	err = templates.TeamActivity(user.CurrentInstance.Settings, *team, notifications, locations).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("TeamActivity: rendering template", "error", err)
	}
}
