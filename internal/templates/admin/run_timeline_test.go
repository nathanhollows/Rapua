package templates

import (
	"fmt"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func TestRunElapsed(t *testing.T) {
	assert.Empty(t, runElapsed(models.Run{}), "no start time means no elapsed duration")
}

func TestFormatDuration(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{12 * time.Minute, "12m"},
		{time.Hour, "1h"},
		{64 * time.Minute, "1h 4m"},
		{48 * time.Hour, "2d"},
		{51 * time.Hour, "2d 3h"},
	} {
		assert.Equal(t, tc.want, formatDuration(tc.in), tc.in.String())
	}
}

// runFixture pairs a run with its completed-objective count, since
// countFinished and medianProgress key their completedCounts lookup by run
// code and never read any other run field.
type runFixture struct {
	started   bool
	completed int
}

func runsWith(fixtures []runFixture) ([]models.Run, map[string]int) {
	runs := make([]models.Run, 0, len(fixtures))
	counts := make(map[string]int, len(fixtures))
	for i, f := range fixtures {
		run := models.Run{HasStarted: f.started, Code: fmt.Sprintf("R%d", i)}
		runs = append(runs, run)
		counts[run.Code] = f.completed
	}
	return runs, counts
}

func TestCountNotStarted(t *testing.T) {
	runs, _ := runsWith([]runFixture{
		{started: true, completed: 3},
		{started: false},
		{started: false},
	})
	assert.Equal(t, 2, countNotStarted(runs))
	assert.Zero(t, countNotStarted(nil))
}

func TestCountFinished(t *testing.T) {
	runs, counts := runsWith([]runFixture{
		{started: true, completed: 7},
		{started: true, completed: 3},
		{started: true, completed: 8},
	})

	assert.Equal(t, 2, countFinished(runs, 7, counts), "7 of 7 and 8 of 7 both count as finished")
	assert.Zero(t, countFinished(runs, 0, counts), "no objectives means nothing can be finished")
}

func TestMedianProgress(t *testing.T) {
	t.Run("odd count takes the middle", func(t *testing.T) {
		runs, counts := runsWith([]runFixture{
			{started: true, completed: 1},
			{started: true, completed: 9},
			{started: true, completed: 5},
		})
		assert.Equal(t, 5, medianProgress(runs, counts))
	})

	t.Run("even count averages the two middles", func(t *testing.T) {
		runs, counts := runsWith([]runFixture{
			{started: true, completed: 2},
			{started: true, completed: 4},
			{started: true, completed: 6},
			{started: true, completed: 8},
		})
		assert.Equal(t, 5, medianProgress(runs, counts))
	})

	t.Run("unstarted runs are excluded", func(t *testing.T) {
		// The reason for median over mean: three dormant runs should not drag
		// the figure toward zero.
		runs, counts := runsWith([]runFixture{
			{started: false},
			{started: false},
			{started: false},
			{started: true, completed: 6},
			{started: true, completed: 6},
			{started: true, completed: 6},
		})
		assert.Equal(t, 6, medianProgress(runs, counts))
	})

	t.Run("no started runs", func(t *testing.T) {
		unstarted, _ := runsWith([]runFixture{{started: false}})
		assert.Zero(t, medianProgress(unstarted, map[string]int{}))
		assert.Zero(t, medianProgress(nil, nil))
	})
}

func TestCountUnread(t *testing.T) {
	assert.Equal(t, 2, countUnread([]models.Notification{
		{Dismissed: false}, {Dismissed: true}, {Dismissed: false},
	}))
	assert.Zero(t, countUnread(nil))
}
