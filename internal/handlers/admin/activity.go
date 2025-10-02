package admin

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v4/models"
)

// Activity displays the activity tracker page.
func (h *AdminHandler) Activity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i := range user.CurrentInstance.Teams {
		if user.CurrentInstance.Teams[i].Code == "" {
			continue // Skip teams without a code
		}
		err := h.teamService.LoadRelation(r.Context(), &user.CurrentInstance.Teams[i], "Scans")
		if err != nil {
			h.handleError(
				w,
				r,
				"ActivityTeamsOverview: loading team relations",
				"Error loading team relations",
				"Could not load data",
				err,
			)
			return
		}
	}

	c := templates.ActivityTracker(user.CurrentInstance)
	err := templates.Layout(c, *user, "Activity", "Activity").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Activity: rendering template", "error", err)
	}
}

// ActivityTeamsOverview displays the activity tracker page.
func (h *AdminHandler) ActivityTeamsOverview(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teams := filterTeamsStarted(user.CurrentInstance.Teams)

	for i := range teams {
		err := h.teamService.LoadRelation(r.Context(), &teams[i], "Scans")
		if err != nil {
			h.handleError(
				w,
				r,
				"ActivityTeamsOverview: loading team relations",
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
		len(user.CurrentInstance.Locations),
		rankingScheme,
		sortField,
		sortOrder,
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"ActivityTeamsOverview: getting leaderboard data",
			"Error getting leaderboard data",
			"Could not load data",
			err,
		)
		return
	}

	err = templates.ActivityTeamsTable(user.CurrentInstance.Settings, len(user.CurrentInstance.Locations), leaderboardData, sortField, sortOrder).
		Render(r.Context(), w)
	if err != nil {
		h.logger.Error("ActivityTeamsOverview: rendering template", "error", err)
	}
}

func filterTeamsStarted(teams []models.Team) []models.Team {
	var filtered []models.Team
	for _, team := range teams {
		if team.HasStarted {
			filtered = append(filtered, team)
		}
	}
	return filtered
}

// TeamActivity displays the activity tracker page.
// It accepts HTMX requests to update the team activity.
func (h *AdminHandler) TeamActivity(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	teamCode := chi.URLParam(r, "teamCode")

	team, err := h.teamService.GetTeamByCode(r.Context(), teamCode)
	if err != nil || team.InstanceID != user.CurrentInstanceID {
		h.handleError(w, r, "TeamActivity: getting team", "Error getting team", "Could not load data", err)
		return
	}

	err = h.teamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "TeamActivity: loading scans", "Error loading data", "Could not load data", err)
		return
	}

	locations, err := h.navigationService.GetNextLocations(r.Context(), team)
	if err != nil {
		if !errors.Is(err, services.ErrAllLocationsVisited) {
			h.handleError(
				w,
				r,
				"TeamActivity: getting next locations",
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
			"TeamActivity: getting notifications",
			"Error getting notifications",
			"Could not load data",
			err,
		)
		return
	}

	err = templates.TeamActivity(user.CurrentInstance.Settings, *team, notifications, locations).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamActivity: rendering template", "error", err)
	}
}
