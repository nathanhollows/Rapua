package players

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// A quest keeping score ranks by it. A quest that is not has no score to rank
// by, so it ranks by how much of the quest each team has done: the board stays
// meaningful either way without the author choosing a scheme.
func TestLeaderboardScheme_FollowsPoints(t *testing.T) {
	assert.Equal(t,
		string(services.RankByPoints),
		leaderboardScheme(models.QuestSettings{EnablePoints: true}),
	)
	assert.Equal(t,
		string(services.RankByProgress),
		leaderboardScheme(models.QuestSettings{EnablePoints: false}),
	)
}

// A name longer than a leaderboard row can show is cut here, because a form's
// maxlength only binds the browser.
func TestTeamName_IsCutToWhatARowCanShow(t *testing.T) {
	long := strings.Repeat("x", teamNameLimit+10)
	assert.Len(t, []rune(trimTeamName(long)), teamNameLimit)
	assert.Equal(t, "The Quokkas", trimTeamName("  The Quokkas  "))
	assert.Empty(t, trimTeamName("   "), "a blank name returns a team to its code")
}
