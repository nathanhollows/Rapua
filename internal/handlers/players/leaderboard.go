package players

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
)

// leaderboardScheme picks how the board is ordered from what the quest counts.
//
// A quest keeping score ranks by it. One that is not has no score to rank by,
// so it ranks by how far through the quest each team is, which is the only
// other thing every run has. Either way the board means something without the
// author choosing a scheme.
func leaderboardScheme(settings models.QuestSettings) string {
	if settings.EnablePoints {
		return string(services.RankByPoints)
	}
	return string(services.RankByProgress)
}

// leaderboard builds the board a player sees, or nothing when the quest shows
// no board or it cannot be built: a board is not worth failing the team page
// over, and the page says who you are playing as whether or not anyone is
// keeping score.
func (h *PlayerHandler) leaderboard(
	r *http.Request, team *models.Run,
) []services.LeaderBoardTeamData {
	if !team.Quest.Settings.ShowLeaderboard {
		return nil
	}

	teams, err := h.runService.FindAll(r.Context(), team.QuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "leaderboard: loading runs", "error", err)
		return nil
	}
	completed, err := h.runService.CountCompletedObjectivesByRun(r.Context(), team.QuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "leaderboard: counting completions", "error", err)
		return nil
	}

	// Ranked, and sorted by that rank: a player is shown one board rather than
	// the admin's sortable table, so nothing here comes from the URL.
	board, err := h.leaderBoardService.GetLeaderBoardData(
		r.Context(),
		teams,
		len(team.Quest.Objectives),
		completed,
		leaderboardScheme(team.Quest.Settings),
		"rank",
		"asc",
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "leaderboard: ranking", "error", err)
		return nil
	}
	return board
}
