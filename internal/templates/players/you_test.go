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

// Points and the leaderboard are separate settings.
func TestYou_ShowsPointsWithoutALeaderboard(t *testing.T) {
	html := render(t, You(YouParams{Run: pointsRun(140, true)}))

	assert.Contains(t, html, ">Points</span>")
	assert.Contains(t, html, "140")
	assert.NotContains(t, html, ">Position</span>", "nothing to be positioned against")
	assert.NotContains(t, html, "<h2")
}

// With the board off and no score, there is nothing to report but identity.
func TestYou_ShowsOnlyIdentityWhenNothingIsCounted(t *testing.T) {
	html := render(t, You(YouParams{Run: pointsRun(0, false)}))

	assert.Contains(t, html, ">Game code</span>")
	assert.NotContains(t, html, ">Points</span>")
	assert.NotContains(t, html, ">Position</span>")
}

// Position needs a board to be a position in.
func TestYou_ShowsPositionOnlyWithABoard(t *testing.T) {
	run := pointsRun(140, true)
	html := render(t, You(YouParams{
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

// Progress counts against the board entry's total, the denominator the rank used.
func TestYou_ProgressCountsAgainstTheBoardTotal(t *testing.T) {
	html := render(t, You(YouParams{
		Run: pointsRun(0, false),
		Leaderboard: []services.LeaderBoardTeamData{
			{Code: "FYHG", Progress: 7, Total: 12, Rank: 1},
		},
	}))

	assert.Contains(t, html, ">Objectives done</span>")
	assert.Contains(t, html, "7")
	assert.Contains(t, html, "of 12")
}

func TestYou_TheGameCodeIsOneTilePerLetterAndReadAsLetters(t *testing.T) {
	html := render(t, You(YouParams{Run: pointsRun(0, false)}))

	assert.Equal(t, 4, strings.Count(html, `class="q-char"`))
	assert.Contains(t, html, `aria-label="Game code F Y H G"`)
}

// A stat drawn like a button invites a tap that goes nowhere.
func TestYou_OnlyTheRenameIsAButton(t *testing.T) {
	html := render(t, You(YouParams{Run: pointsRun(140, true)}))

	assert.Equal(t, 1, strings.Count(html, "<button"), "the pencil and nothing else")
	assert.Contains(t, html, `aria-label="Rename your team"`)
}

func TestYou_TheBoardMarksYouAndNoOneElse(t *testing.T) {
	html := render(t, You(YouParams{
		Run: pointsRun(140, true),
		Leaderboard: []services.LeaderBoardTeamData{
			{Code: "AAAA", Name: "Kererū", Points: 210, Rank: 1},
			{Code: "FYHG", Points: 140, Rank: 2},
		},
	}))

	assert.Equal(t, 1, strings.Count(html, "q-lb-me"))
	assert.Equal(t, 1, strings.Count(html, ">you</em>"))
}
