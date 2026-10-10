package templates

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func pointsRun(points int, enabled bool) models.Run {
	return models.Run{
		Code:   "FYHG",
		Points: points,
		Quest:  models.Quest{Settings: models.QuestSettings{EnablePoints: enabled}},
	}
}

// Points and the leaderboard are separate settings. A quest can keep score
// without showing anyone else's, and the score is the player's own either way.
func TestTeam_ShowsPointsWithoutALeaderboard(t *testing.T) {
	html := render(t, Team(TeamParams{Run: pointsRun(140, true)}))

	assert.Contains(t, html, ">Points</span>")
	assert.Contains(t, html, "140")
	assert.NotContains(t, html, ">Position</span>", "nothing to be positioned against")
	assert.NotContains(t, html, "<h2")
}

// With the board off and no score, there is nothing to report but identity.
func TestTeam_ShowsOnlyIdentityWhenNothingIsCounted(t *testing.T) {
	html := render(t, Team(TeamParams{Run: pointsRun(0, false)}))

	assert.Contains(t, html, ">Join code</span>")
	assert.NotContains(t, html, ">Points</span>")
	assert.NotContains(t, html, ">Position</span>")
}

// Position needs a board to be a position in.
func TestTeam_ShowsPositionOnlyWithABoard(t *testing.T) {
	run := pointsRun(140, true)
	html := render(t, Team(TeamParams{
		Run: run,
		Leaderboard: []services.LeaderBoardTeamData{
			{Code: "AAAA", Name: "Kererū", Points: 210, Rank: 1},
			{Code: "FYHG", Points: 140, Rank: 2},
		},
	}))

	assert.Contains(t, html, ">Position</span>")
	assert.Contains(t, html, "2nd")
	assert.Contains(t, html, "of 2")
	assert.Equal(t, 1, strings.Count(html, ">Points</span>"), "the score is said once")
}
