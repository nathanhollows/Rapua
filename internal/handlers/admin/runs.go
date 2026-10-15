package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	admin "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

func (h *Handler) Runs(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Status is a bonus on a list about codes and access, so degrade rather than fail.
	summaries, err := h.runService.GetQuestRunSummaries(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.logger.WarnContext(r.Context(), "Runs: building run summaries", "error", err)
		summaries = nil
	}

	c := admin.Teams(user.CurrentQuest.Runs, user.FreeCredits+user.PaidCredits, summaries)
	err = admin.Layout(c, *user, "Runs", "Runs").Render(r.Context(), w)

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

	// A new run has no stream yet, so it draws as not started.
	err = admin.TeamsList(teams, nil).Render(r.Context(), w)
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

	// A filter click swaps the card alone, keeping scroll and skipping the other queries.
	if r.Header.Get("Hx-Target") == "run-log-card" {
		runLog, logErr := h.runService.GetRunLog(r.Context(), &user.CurrentQuest, team)
		if logErr != nil {
			h.handleError(
				w, r, "TeamOverview: building the run log", "Error loading data", "Could not load data", logErr,
			)
			return
		}
		err = admin.RunLogCard(runLog, r.URL.Query().Get("show"), team.Code).Render(r.Context(), w)
		if err != nil {
			h.logger.ErrorContext(r.Context(), "rendering the run log card", "error", err.Error())
		}
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "TeamOverview: loading scans", "Error loading data", "Could not load data", err)
		return
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

	progress, err := h.runService.GetRunProgress(r.Context(), user.CurrentQuestID, team.Code)
	if err != nil {
		h.handleError(w, r, "TeamOverview: loading progress", "Error loading data", "Could not load data", err)
		return
	}

	uploads := h.runOverviewUploads(r.Context(), team)

	runLog, err := h.runService.GetRunLog(r.Context(), &user.CurrentQuest, team)
	if err != nil {
		h.handleError(w, r, "TeamOverview: building the run log", "Error loading data", "Could not load data", err)
		return
	}

	data := admin.TeamOverviewData{
		Quest:         user.CurrentQuest,
		Run:           *team,
		Notifications: notifications,
		Uploads:       uploads,
		Progress:      progress,
		RunLog:        runLog,
		Show:          r.URL.Query().Get("show"),
	}
	c := admin.TeamOverview(data)
	title := team.Name
	if title == "" {
		title = "Run " + team.Code
	}
	err = admin.Layout(c, *user, "Runs", title).Render(r.Context(), w)
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

// runOverviewUploads degrades to empty: uploads should not cost the admin the page.
func (h *Handler) runOverviewUploads(ctx context.Context, team *models.Run) []*models.Upload {
	uploads, err := h.uploadService.Search(ctx, map[string]string{"run_code": team.Code})
	if err != nil {
		h.logger.WarnContext(ctx, "TeamOverview: failed to load uploads", "error", err, "run_code", team.Code)
		return []*models.Upload{}
	}
	return uploads
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
