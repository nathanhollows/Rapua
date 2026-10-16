package templates

import (
	"fmt"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
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

// runFixture pairs a run with its line from the quest's summaries, since the
// stats helpers read nothing else: not the tree, and not the objective count.
type runFixture struct {
	started bool
	done    int
	total   int
	status  services.TeamStatus
}

func runsWith(fixtures []runFixture) ([]models.Run, map[string]services.RunSummary) {
	runs := make([]models.Run, 0, len(fixtures))
	summaries := make(map[string]services.RunSummary, len(fixtures))
	for i, f := range fixtures {
		run := models.Run{HasStarted: f.started, Code: fmt.Sprintf("R%d", i)}
		runs = append(runs, run)
		summaries[run.Code] = services.RunSummary{Done: f.done, Total: f.total, Status: f.status}
	}
	return runs, summaries
}

func TestCountNotStarted(t *testing.T) {
	runs, _ := runsWith([]runFixture{
		{started: true, done: 3, total: 7},
		{started: false},
		{started: false},
	})
	assert.Equal(t, 2, countNotStarted(runs))
	assert.Zero(t, countNotStarted(nil))
}

func TestCountRunning(t *testing.T) {
	runs, summaries := runsWith([]runFixture{
		{started: true, done: 3, total: 7, status: services.StatusPlaying},
		{started: true, done: 7, total: 7, status: services.StatusFinished},
		{started: false},
	})

	assert.Equal(t, 1, countRunning(runs, summaries), "a finished run is not still running")
	assert.Zero(t, countRunning(nil, nil))
}

func TestCountFinished(t *testing.T) {
	runs, summaries := runsWith([]runFixture{
		{started: true, done: 7, total: 7, status: services.StatusFinished},
		{started: true, done: 3, total: 7, status: services.StatusPlaying},
		// Finished on a branch it cut: fewer done than the quest's objectives,
		// and still finished.
		{started: true, done: 3, total: 3, status: services.StatusFinished},
	})

	assert.Equal(t, 2, countFinished(runs, summaries), "the run's own end, not every objective")
	assert.Zero(t, countFinished(nil, nil))
}

func TestMedianShare(t *testing.T) {
	t.Run("odd count takes the middle", func(t *testing.T) {
		runs, summaries := runsWith([]runFixture{
			{started: true, done: 1, total: 4},
			{started: true, done: 9, total: 10},
			{started: true, done: 5, total: 10},
		})
		assert.Equal(t, 50, medianShare(runs, summaries))
	})

	t.Run("even count averages the two middles", func(t *testing.T) {
		runs, summaries := runsWith([]runFixture{
			{started: true, done: 2, total: 10},
			{started: true, done: 4, total: 10},
			{started: true, done: 6, total: 10},
			{started: true, done: 8, total: 10},
		})
		assert.Equal(t, 50, medianShare(runs, summaries))
	})

	t.Run("each run is read against what it owes", func(t *testing.T) {
		// Equal counts, different quests' worth of work: the shares are what
		// the median is taken over, not the raw totals.
		runs, summaries := runsWith([]runFixture{
			{started: true, done: 3, total: 3},
			{started: true, done: 3, total: 12},
			{started: true, done: 6, total: 12},
		})
		assert.Equal(t, 50, medianShare(runs, summaries))
	})

	t.Run("unstarted runs are excluded", func(t *testing.T) {
		// The reason for median over mean: three dormant runs should not drag
		// the figure toward zero.
		runs, summaries := runsWith([]runFixture{
			{started: false},
			{started: false},
			{started: false},
			{started: true, done: 6, total: 10},
			{started: true, done: 6, total: 10},
			{started: true, done: 6, total: 10},
		})
		assert.Equal(t, 60, medianShare(runs, summaries))
	})

	t.Run("a run that owes nothing sits it out", func(t *testing.T) {
		runs, summaries := runsWith([]runFixture{
			{started: true, done: 0, total: 0, status: services.StatusStalled},
			{started: true, done: 1, total: 4},
		})
		assert.Equal(t, 25, medianShare(runs, summaries))
	})

	t.Run("no started runs", func(t *testing.T) {
		unstarted, _ := runsWith([]runFixture{{started: false}})
		assert.Zero(t, medianShare(unstarted, map[string]services.RunSummary{}))
		assert.Zero(t, medianShare(nil, nil))
	})
}

func TestCountUnread(t *testing.T) {
	assert.Equal(t, 2, countUnread([]models.Notification{
		{Dismissed: false}, {Dismissed: true}, {Dismissed: false},
	}))
	assert.Zero(t, countUnread(nil))
}
