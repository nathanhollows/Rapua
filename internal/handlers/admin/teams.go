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

func (h *Handler) TeamDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	teamCode := chi.URLParam(r, "teamCode")

	// Verify team exists and belongs to current instance
	team, err := h.teamService.GetTeamByCode(r.Context(), teamCode)
	if err != nil || team == nil || team.InstanceID != user.CurrentInstanceID {
		h.handleError(
			w,
			r,
			"TeamDelete: team not found or access denied",
			"Error deleting team",
			"error",
			err,
			"team_code",
			teamCode,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Delete the team
	if err := h.deleteService.DeleteTeams(r.Context(), user.CurrentInstanceID, []string{teamCode}); err != nil {
		h.handleError(
			w,
			r,
			"TeamDelete: deleting team",
			"Error deleting team",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
			"team_code",
			teamCode,
		)
		return
	}

	h.redirect(w, r, "/admin/")
}

func (h *Handler) TeamReset(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	teamCode := chi.URLParam(r, "teamCode")

	// Verify team exists and belongs to current instance
	team, err := h.teamService.GetTeamByCode(r.Context(), teamCode)
	if err != nil || team == nil || team.InstanceID != user.CurrentInstanceID {
		h.handleError(
			w,
			r,
			"TeamReset: team not found or access denied",
			"Error resetting team",
			"error",
			err,
			"team_code",
			teamCode,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Reset the team
	err = h.deleteService.ResetTeams(r.Context(), user.CurrentInstanceID, []string{teamCode})
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamReset: resetting team",
			"Error resetting team",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
			"team_code",
			teamCode,
		)
		return
	}

	h.redirect(w, r, "/admin/teams")
}
