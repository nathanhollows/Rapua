package admin

import (
	"net/http"

	"github.com/go-chi/chi"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Activity displays the activity tracker page.
func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	completedCounts, err := h.runService.CountCompletedObjectivesByRun(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.handleError(
			w,
			r,
			"Activity: counting completed objectives",
			"Error loading team relations",
			"Could not load data",
			err,
		)
		return
	}

	c := templates.ActivityTracker(user.CurrentQuest, completedCounts)
	err = templates.Layout(c, *user, "Activity", "Activity").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Activity: rendering template", "error", err)
	}
}

// ActivityRunsOverview displays the activity tracker page.
func (h *Handler) ActivityRunsOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teams := filterTeamsStarted(user.CurrentQuest.Runs)

	// Get query parameters for sorting with defaults
	sortField := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")
	rankingScheme := r.URL.Query().Get("ranking")

	// Set defaults if not provided
	if sortField == "" {
		sortField = "rank"
	}
	if sortOrder == "" {
		sortOrder = "asc"
	}

	completedCounts, err := h.runService.CountCompletedObjectivesByRun(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.handleError(
			w,
			r,
			"ActivityRunsOverview: counting completed objectives",
			"Error getting leaderboard data",
			"Could not load data",
			err,
		)
		return
	}

	// Get leaderboard data using the new service
	leaderboardData, err := h.leaderBoardService.GetLeaderBoardData(
		r.Context(),
		teams,
		len(user.CurrentQuest.Objectives),
		completedCounts,
		rankingScheme,
		sortField,
		sortOrder,
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"ActivityRunsOverview: getting leaderboard data",
			"Error getting leaderboard data",
			"Could not load data",
			err,
		)
		return
	}

	err = templates.ActivityTeamsTable(user.CurrentQuest.Settings, len(user.CurrentQuest.Objectives), leaderboardData, sortField, sortOrder).
		Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ActivityRunsOverview: rendering template", "error", err)
	}
}

func filterTeamsStarted(teams []models.Run) []models.Run {
	var filtered []models.Run
	for _, team := range teams {
		if team.HasStarted {
			filtered = append(filtered, team)
		}
	}
	return filtered
}

func subtractObjectivesByID(all, exclude []models.Objective) []models.Objective {
	excludeIDs := make(map[string]bool, len(exclude))
	for _, o := range exclude {
		excludeIDs[o.ID] = true
	}
	remaining := make([]models.Objective, 0, len(all))
	for _, o := range all {
		if !excludeIDs[o.ID] {
			remaining = append(remaining, o)
		}
	}
	return remaining
}

// ActivityStats returns just the stats component for HTMX updates.
func (h *Handler) ActivityStats(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	completedCounts, err := h.runService.CountCompletedObjectivesByRun(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ActivityStats: counting completed objectives", "error", err)
		return
	}

	err = templates.ActivityStats(user.CurrentQuest, completedCounts).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ActivityStats: rendering template", "error", err)
	}
}

// RunActivity displays the activity tracker page.
// It accepts HTMX requests to update the team activity.
func (h *Handler) RunActivity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	runCode := chi.URLParam(r, "runCode")

	team, err := h.runService.GetRunByCode(r.Context(), runCode)
	if err != nil || team.QuestID != user.CurrentQuestID {
		h.handleError(w, r, "RunActivity: getting team", "Error getting team", "Could not load data", err)
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "RunActivity: loading scans", "Error loading data", "Could not load data", err)
		return
	}

	incompleteObjectives, err := h.runService.GetIncompleteObjectives(r.Context(), user.CurrentQuestID, team.Code)
	if err != nil {
		h.handleError(
			w,
			r,
			"RunActivity: getting incomplete objectives",
			"Error getting incomplete objectives",
			"Could not load data",
			err,
		)
		return
	}

	notifications, err := h.notificationService.GetNotifications(r.Context(), team.Code)
	if err != nil {
		h.handleError(
			w,
			r,
			"RunActivity: getting notifications",
			"Error getting notifications",
			"Could not load data",
			err,
		)
		return
	}

	completedObjectives := subtractObjectivesByID(user.CurrentQuest.Objectives, incompleteObjectives)

	err = templates.RunActivity(
		user.CurrentQuest.Settings, *team, notifications, incompleteObjectives, completedObjectives,
	).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "RunActivity: rendering template", "error", err)
	}
}
