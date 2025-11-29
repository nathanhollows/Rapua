package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	admin "github.com/nathanhollows/Rapua/v6/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v6/models"
)

func (h *Handler) Teams(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := admin.Teams(user.CurrentInstance.Teams, user.FreeCredits+user.PaidCredits)
	err := admin.Layout(c, *user, "Teams", "Teams").Render(r.Context(), w)

	if err != nil {
		h.logger.Error("rendering teams page", "error", err.Error())
	}
}

func (h *Handler) TeamsAdd(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"TeamsAdd parsing form",
			"Error adding teams",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	countStr := r.FormValue("count")
	count, err := strconv.Atoi(countStr)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamsAdd parsing count",
			"Error adding teams",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Add the teams
	teams, err := h.teamService.AddTeams(r.Context(), user.CurrentInstanceID, count)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamsAdd adding teams",
			"Error adding teams",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = admin.TeamsList(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamsAdd rendering teams list", "error", err.Error(), "instance_id", user.CurrentInstanceID)
	}
}

func (h *Handler) TeamsDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	teamID := r.Form["team-checkbox"]
	if len(teamID) == 0 {
		h.handleError(
			w,
			r,
			"TeamsDelete no team_id",
			"Error deleting team",
			"error",
			nil,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	for _, id := range teamID {
		if deleteErr := h.deleteService.DeleteTeams(r.Context(), user.CurrentInstanceID, []string{id}); deleteErr != nil {
			h.handleError(
				w,
				r,
				"TeamsDelete deleting team",
				"Error deleting team",
				"error",
				deleteErr,
				"instance_id",
				user.CurrentInstanceID,
				"team_id",
				teamID,
			)
			return
		}
	}

	teams, err := h.teamService.FindAll(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamsReset finding teams",
			"Error finding teams",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = admin.TeamsTable(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamsReset rendering teams list", "error", err.Error(), "instance_id", user.CurrentInstanceID)
	}
	h.handleSuccess(w, r, "Deleted team(s)")
}

func (h *Handler) TeamsReset(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	teamIDs := r.Form["team-checkbox"]
	if len(teamIDs) == 0 {
		h.handleError(
			w,
			r,
			"TeamsReset no team_id",
			"No teams selected",
			"error",
			nil,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = h.deleteService.ResetTeams(r.Context(), user.CurrentInstanceID, teamIDs)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamsReset deleting team",
			"Error resetting teams",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
			"team_id",
			teamIDs,
		)
		return
	}

	teams, err := h.teamService.FindAll(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamsReset finding teams",
			"Error finding teams",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = admin.TeamsTable(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamsReset rendering teams list", "error", err.Error(), "instance_id", user.CurrentInstanceID)
	}
	h.handleSuccess(w, r, "Reset team(s)")
}

func (h *Handler) TeamOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teamCode := chi.URLParam(r, "teamCode")

	team, err := h.teamService.GetTeamByCode(r.Context(), teamCode)
	if err != nil || team == nil || team.InstanceID != user.CurrentInstanceID {
		h.logger.Warn(
			"TeamOverview: team not found or access denied",
			"error",
			err,
			"team_code",
			teamCode,
			"instance_id",
			user.CurrentInstanceID,
		)
		http.Redirect(w, r, "/admin/teams", http.StatusSeeOther)
		return
	}

	err = h.teamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "TeamOverview: loading scans", "Error loading data", "Could not load data", err)
		return
	}

	locations, err := h.navigationService.GetNextLocations(r.Context(), team)
	if err != nil {
		if !errors.Is(err, services.ErrAllLocationsVisited) {
			h.handleError(
				w,
				r,
				"TeamOverview: getting next locations",
				"Error getting next locations",
				"Could not load data",
				err,
			)
			return
		}
	}

	notifications, err := h.notificationService.GetNotifications(r.Context(), team.Code)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamOverview: getting notifications",
			"Error getting notifications",
			"Could not load data",
			err,
		)
		return
	}

	// Get uploads for this team
	uploads, err := h.uploadService.Search(r.Context(), map[string]string{
		"team_code": team.Code,
	})
	if err != nil {
		h.logger.Warn("TeamOverview: failed to load uploads", "error", err, "team_code", team.Code)
		// Continue without uploads rather than failing completely
		uploads = []*models.Upload{}
	}

	// Build location to group mapping
	locationGroups := h.teamService.BuildLocationGroupMap(&user.CurrentInstance.GameStructure)

	// Build group order mapping
	groupOrder := h.teamService.BuildGroupOrder(&user.CurrentInstance.GameStructure)

	// Group check-ins by their location group
	groupedHistory := h.teamService.GroupCheckInsByGroup(team.CheckIns, locationGroups, groupOrder)

	data := admin.TeamOverviewData{
		Instance:       user.CurrentInstance,
		Team:           *team,
		Notifications:  notifications,
		NextLocations:  locations,
		Uploads:        uploads,
		TotalLocations: len(user.CurrentInstance.Locations),
		LocationGroups: locationGroups,
		GroupedHistory: groupedHistory,
	}
	c := admin.TeamOverview(data)
	err = admin.Layout(c, *user, "Teams", "Team Overview").Render(r.Context(), w)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamOverview: rendering template",
			"Error rendering page",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}
}
