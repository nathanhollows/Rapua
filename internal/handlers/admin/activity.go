package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/internal/flash"
	"github.com/nathanhollows/Rapua/internal/models"
	"github.com/nathanhollows/Rapua/internal/services"
	templates "github.com/nathanhollows/Rapua/internal/templates/admin"
)

// Activity displays the activity tracker page
func (h *AdminHandler) Activity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.GameManagerService.LoadTeams(r.Context(), &user.CurrentInstance.Teams)
	if err != nil {
		h.Logger.Error("Activity: loading teams", "error", err)
	}

	c := templates.ActivityTracker(*user)
	err = templates.Layout(c, *user, "Activity", "Activity").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("Activity: rendering template", "error", err)
	}
}

// ActivityTeamsOverview displays the activity tracker page
func (h *AdminHandler) ActivityTeamsOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.GameManagerService.LoadTeams(r.Context(), &user.CurrentInstance.Teams)
	if err != nil {
		h.Logger.Error("Activity: loading teams", "error", err)
	}

	err = templates.ActivityTeamsTable(user.CurrentInstance.Locations, user.CurrentInstance.Teams).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("ActivityTeamsOverview: rendering template", "error", err)
	}
}

// TeamActivity displays the activity tracker page
// It accepts HTMX requests to update the team activity
func (h *AdminHandler) TeamActivity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teamCode := chi.URLParam(r, "teamCode")

	gameplayService := &services.GameplayService{}
	team, err := gameplayService.GetTeamByCode(r.Context(), teamCode)
	if err != nil || team.InstanceID != user.CurrentInstanceID {
		h.Logger.Error("TeamActivity: team not found", "error", err, "instanceID", user.CurrentInstanceID, "teamCode", chi.URLParam(r, "teamCode"))
		err := templates.Toast(*flash.NewError("Team not found")).Render(r.Context(), w)
		if err != nil {
			h.Logger.Error("TeamActivity: rendering template", "error", err)
		}
		return
	}

	team.LoadScans(r.Context())
	response := gameplayService.SuggestNextLocations(r.Context(), team, user.CurrentInstance.Settings.MaxNextLocations)
	if response.Error != nil {
		http.Error(w, response.Error.Error(), http.StatusInternalServerError)
		return
	}

	notifications, err := h.NotificationService.GetNotifications(r.Context(), team.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var nextLocations models.Locations
	if response.Data["nextLocations"] == nil {
		nextLocations = models.Locations{}
	} else {
		nextLocations = response.Data["nextLocations"].(models.Locations)
	}

	err = templates.TeamActivity(user.CurrentInstance.Settings, *team, notifications, nextLocations).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("TeamActivity: rendering template", "error", err)
	}

}
