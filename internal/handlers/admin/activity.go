package admin

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v7/models"
)

// Activity displays the activity tracker page.
func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i := range user.CurrentQuest.Runs {
		if user.CurrentQuest.Runs[i].Code == "" {
			continue // Skip teams without a code
		}
		err := h.runService.LoadCheckIns(r.Context(), &user.CurrentQuest.Runs[i])
		if err != nil {
			h.handleError(
				w,
				r,
				"ActivityRunsOverview: loading team relations",
				"Error loading team relations",
				"Could not load data",
				err,
			)
			return
		}
	}

	c := templates.ActivityTracker(user.CurrentQuest)
	err := templates.Layout(c, *user, "Activity", "Activity").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Activity: rendering template", "error", err)
	}
}

// ActivityRunsOverview displays the activity tracker page.
func (h *Handler) ActivityRunsOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teams := filterTeamsStarted(user.CurrentQuest.Runs)

	for i := range teams {
		err := h.runService.LoadCheckIns(r.Context(), &teams[i])
		if err != nil {
			h.handleError(
				w,
				r,
				"ActivityRunsOverview: loading team relations",
				"Error loading team relations",
				"Could not load data",
				err,
			)
			return
		}
	}

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

	// Get leaderboard data using the new service
	leaderboardData, err := h.leaderBoardService.GetLeaderBoardData(
		r.Context(),
		teams,
		len(user.CurrentQuest.Locations),
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

	err = templates.ActivityTeamsTable(user.CurrentQuest.Settings, len(user.CurrentQuest.Locations), leaderboardData, sortField, sortOrder).
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

// ActivityStats returns just the stats component for HTMX updates.
func (h *Handler) ActivityStats(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i := range user.CurrentQuest.Runs {
		if user.CurrentQuest.Runs[i].Code == "" {
			continue
		}
		err := h.runService.LoadCheckIns(r.Context(), &user.CurrentQuest.Runs[i])
		if err != nil {
			h.logger.ErrorContext(r.Context(), "ActivityStats: loading team relations", "error", err)
			return
		}
	}

	err := templates.ActivityStats(user.CurrentQuest).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ActivityStats: rendering template", "error", err)
	}
}

// ActivityLocations returns just the location overview list for HTMX updates.
func (h *Handler) ActivityLocations(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i := range user.CurrentQuest.Runs {
		if user.CurrentQuest.Runs[i].Code == "" {
			continue
		}
		err := h.runService.LoadCheckIns(r.Context(), &user.CurrentQuest.Runs[i])
		if err != nil {
			h.logger.ErrorContext(r.Context(), "ActivityLocations: loading team relations", "error", err)
			return
		}
	}

	err := templates.LocationOverviewList(user.CurrentQuest).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "ActivityLocations: rendering template", "error", err)
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

	locations, err := h.navigationService.GetNextLocations(r.Context(), team)
	if err != nil {
		if !errors.Is(err, services.ErrAllLocationsVisited) {
			h.handleError(
				w,
				r,
				"RunActivity: getting next locations",
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
			"RunActivity: getting notifications",
			"Error getting notifications",
			"Could not load data",
			err,
		)
		return
	}

	err = templates.RunActivity(user.CurrentQuest.Settings, *team, notifications, locations).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "RunActivity: rendering template", "error", err)
	}
}
