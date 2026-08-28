package templates

import (
	"fmt"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func checkIn(locationID, name string, timeIn time.Time, points int) models.CheckIn {
	return models.CheckIn{
		LocationID: locationID,
		TimeIn:     timeIn,
		Points:     points,
		Location:   models.Location{ID: locationID, Name: name},
	}
}

func TestBuildRunTimeline_OrdersOldestFirst(t *testing.T) {
	start := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)

	// The repository returns check-ins time_in DESC — feed them in that order
	// to prove the builder does not depend on the incoming order.
	run := models.Run{
		StartedAt: start,
		CheckIns: []models.CheckIn{
			checkIn("c", "Third", start.Add(30*time.Minute), 5),
			checkIn("b", "Second", start.Add(20*time.Minute), 10),
			checkIn("a", "First", start.Add(5*time.Minute), 20),
		},
	}

	entries := buildRunTimeline(run)

	assert.Len(t, entries, 4, "start marker plus three check-ins")
	assert.Equal(t, TimelineStart, entries[0].Kind)
	assert.Equal(t, []string{"", "First", "Second", "Third"},
		[]string{entries[0].Label, entries[1].Label, entries[2].Label, entries[3].Label})
}

func TestBuildRunTimeline_GapsAreBetweenConsecutiveEvents(t *testing.T) {
	start := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	run := models.Run{
		StartedAt: start,
		CheckIns: []models.CheckIn{
			checkIn("a", "First", start.Add(5*time.Minute), 0),
			checkIn("b", "Second", start.Add(20*time.Minute), 0),
		},
	}

	entries := buildRunTimeline(run)

	assert.Zero(t, entries[0].GapBefore, "start marker has no preceding event")
	assert.Equal(t, 5*time.Minute, entries[1].GapBefore, "measured from run start")
	assert.Equal(t, 15*time.Minute, entries[2].GapBefore, "measured from previous check-in")
}

func TestBuildRunTimeline_MarksBlockingLocationAsCurrent(t *testing.T) {
	start := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	run := models.Run{
		StartedAt:    start,
		MustCheckOut: "b",
		CheckIns: []models.CheckIn{
			checkIn("a", "First", start.Add(5*time.Minute), 0),
			checkIn("b", "Second", start.Add(20*time.Minute), 0),
		},
	}

	entries := buildRunTimeline(run)

	assert.Equal(t, TimelineDone, entries[1].Kind)
	assert.Equal(t, TimelineCurrent, entries[2].Kind)
}

func TestBuildRunTimeline_NotStarted(t *testing.T) {
	assert.Empty(t, buildRunTimeline(models.Run{}), "no start marker and no check-ins")
}

func TestBuildRunTimeline_FallsBackToCreatedAtWhenTimeInMissing(t *testing.T) {
	start := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	older := models.CheckIn{LocationID: "a", Location: models.Location{Name: "Legacy"}}
	older.CreatedAt = start.Add(10 * time.Minute)

	entries := buildRunTimeline(models.Run{StartedAt: start, CheckIns: []models.CheckIn{older}})

	assert.Len(t, entries, 2)
	assert.Equal(t, start.Add(10*time.Minute), entries[1].At)
	assert.Equal(t, 10*time.Minute, entries[1].GapBefore)
}

func TestRunLastSeen_ReturnsNewestRegardlessOfOrder(t *testing.T) {
	start := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)

	// This is the bug the old template had: it read CheckIns[len-1] against a
	// DESC-ordered slice, so it displayed the oldest check-in as "last seen".
	run := models.Run{CheckIns: []models.CheckIn{
		checkIn("c", "Newest", start.Add(30*time.Minute), 0),
		checkIn("a", "Oldest", start.Add(5*time.Minute), 0),
	}}

	at, ok := runLastSeen(run)

	assert.True(t, ok)
	assert.Equal(t, start.Add(30*time.Minute), at)
}

func TestRunLastSeen_NoCheckIns(t *testing.T) {
	_, ok := runLastSeen(models.Run{})
	assert.False(t, ok)
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

// runCodeCounter gives each runWith() fixture a unique Code, since countFinished
// and medianProgress key their completedCounts lookup by run code rather than
// reading run.CheckIns directly.
var runCodeCounter int

func runWith(started bool, checkIns int) models.Run {
	runCodeCounter++
	run := models.Run{HasStarted: started, Code: fmt.Sprintf("R%d", runCodeCounter)}
	for i := 0; i < checkIns; i++ {
		run.CheckIns = append(run.CheckIns, models.CheckIn{})
	}
	return run
}

// countsFromCheckIns bridges these fixtures (which encode progress via CheckIns
// count, the pre-objective way) to the completedCounts map countFinished and
// medianProgress expect, keyed by run code.
func countsFromCheckIns(runs []models.Run) map[string]int {
	counts := make(map[string]int, len(runs))
	for _, run := range runs {
		counts[run.Code] = len(run.CheckIns)
	}
	return counts
}

func TestCountNotStarted(t *testing.T) {
	runs := []models.Run{runWith(true, 3), runWith(false, 0), runWith(false, 0)}
	assert.Equal(t, 2, countNotStarted(runs))
	assert.Zero(t, countNotStarted(nil))
}

func TestCountFinished(t *testing.T) {
	runs := []models.Run{runWith(true, 7), runWith(true, 3), runWith(true, 8)}

	assert.Equal(t, 2, countFinished(runs, 7, countsFromCheckIns(runs)), "7 of 7 and 8 of 7 both count as finished")
	assert.Zero(t, countFinished(runs, 0, countsFromCheckIns(runs)), "no objectives means nothing can be finished")
}

func TestMedianProgress(t *testing.T) {
	t.Run("odd count takes the middle", func(t *testing.T) {
		runs := []models.Run{runWith(true, 1), runWith(true, 9), runWith(true, 5)}
		assert.Equal(t, 5, medianProgress(runs, countsFromCheckIns(runs)))
	})

	t.Run("even count averages the two middles", func(t *testing.T) {
		runs := []models.Run{runWith(true, 2), runWith(true, 4), runWith(true, 6), runWith(true, 8)}
		assert.Equal(t, 5, medianProgress(runs, countsFromCheckIns(runs)))
	})

	t.Run("unstarted runs are excluded", func(t *testing.T) {
		// The reason for median over mean: three dormant runs should not drag
		// the figure toward zero.
		runs := []models.Run{
			runWith(false, 0), runWith(false, 0), runWith(false, 0),
			runWith(true, 6), runWith(true, 6), runWith(true, 6),
		}
		assert.Equal(t, 6, medianProgress(runs, countsFromCheckIns(runs)))
	})

	t.Run("no started runs", func(t *testing.T) {
		unstarted := []models.Run{runWith(false, 0)}
		assert.Zero(t, medianProgress(unstarted, countsFromCheckIns(unstarted)))
		assert.Zero(t, medianProgress(nil, nil))
	})
}

func TestCountUnread(t *testing.T) {
	assert.Equal(t, 2, countUnread([]models.Notification{
		{Dismissed: false}, {Dismissed: true}, {Dismissed: false},
	}))
	assert.Zero(t, countUnread(nil))
}
