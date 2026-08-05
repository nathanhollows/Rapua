package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	admin "github.com/nathanhollows/Rapua/v7/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v7/models"
)

func (h *Handler) Runs(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := admin.Teams(user.CurrentQuest.Runs, user.FreeCredits+user.PaidCredits)
	err := admin.Layout(c, *user, "Teams", "Teams").Render(r.Context(), w)

	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering teams page", "error", err.Error())
	}
}

func (h *Handler) RunsAdd(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"TeamsAdd parsing form",
			"Error adding teams",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
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
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Add the teams
	teams, err := h.runService.AddTeams(r.Context(), user.CurrentQuestID, count)
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamsAdd adding teams",
			"Error adding teams",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	err = admin.TeamsList(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"TeamsAdd rendering teams list",
			"error",
			err.Error(),
			"quest_id",
			user.CurrentQuestID,
		)
	}
}

func (h *Handler) RunOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	runCode := chi.URLParam(r, "runCode")

	team, err := h.runService.GetRunByCode(r.Context(), runCode)
	if err != nil || team == nil || team.QuestID != user.CurrentQuestID {
		h.logger.WarnContext(r.Context(),
			"TeamOverview: team not found or access denied",
			"error",
			err,
			"run_code",
			runCode,
			"quest_id",
			user.CurrentQuestID,
		)
		http.Redirect(w, r, "/admin/runs", http.StatusSeeOther)
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
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
		"run_code": team.Code,
	})
	if err != nil {
		h.logger.WarnContext(r.Context(), "TeamOverview: failed to load uploads", "error", err, "run_code", team.Code)
		// Continue without uploads rather than failing completely
		uploads = []*models.Upload{}
	}

	// Build location to group mapping
	locationGroups := h.runService.BuildLocationGroupMap(&user.CurrentQuest.GameStructure)

	// Build group order mapping
	groupOrder := h.runService.BuildGroupOrder(&user.CurrentQuest.GameStructure)

	// Group check-ins by their location group
	groupedHistory := h.runService.GroupCheckInsByGroup(team.CheckIns, locationGroups, groupOrder)

	data := admin.TeamOverviewData{
		Quest:          user.CurrentQuest,
		Run:            *team,
		Notifications:  notifications,
		NextLocations:  locations,
		Uploads:        uploads,
		TotalLocations: len(user.CurrentQuest.Locations),
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
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}
}

func (h *Handler) RunDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	runCode := chi.URLParam(r, "runCode")

	// Verify team exists and belongs to current instance
	team, err := h.runService.GetRunByCode(r.Context(), runCode)
	if err != nil || team == nil || team.QuestID != user.CurrentQuestID {
		h.handleError(
			w,
			r,
			"TeamDelete: team not found or access denied",
			"Error deleting team",
			"error",
			err,
			"run_code",
			runCode,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Delete the team
	err = h.deleteService.DeleteTeams(r.Context(), user.CurrentQuestID, []string{runCode})
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamDelete: deleting team",
			"Error deleting team",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"run_code",
			runCode,
		)
		return
	}

	h.redirect(w, r, "/admin/")
}

func (h *Handler) RunReset(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	runCode := chi.URLParam(r, "runCode")

	// Verify team exists and belongs to current instance
	team, err := h.runService.GetRunByCode(r.Context(), runCode)
	if err != nil || team == nil || team.QuestID != user.CurrentQuestID {
		h.handleError(
			w,
			r,
			"TeamReset: team not found or access denied",
			"Error resetting team",
			"error",
			err,
			"run_code",
			runCode,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Reset the team
	err = h.deleteService.ResetTeams(r.Context(), user.CurrentQuestID, []string{runCode})
	if err != nil {
		h.handleError(
			w,
			r,
			"TeamReset: resetting team",
			"Error resetting team",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"run_code",
			runCode,
		)
		return
	}

	h.redirect(w, r, "/admin/runs")
}
