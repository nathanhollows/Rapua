package templates

import (
	"fmt"
	"sort"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
)

type TimelineKind int

const (
	TimelineStart TimelineKind = iota
	TimelineDone
	TimelineCurrent
)

type TimelineEntry struct {
	Kind      TimelineKind
	Label     string
	At        time.Time
	GapBefore time.Duration
	Points    int
}

// Check-ins arrive ordered time_in DESC, so they are re-sorted here.
func buildRunTimeline(run models.Run) []TimelineEntry {
	checkIns := make([]models.CheckIn, len(run.CheckIns))
	copy(checkIns, run.CheckIns)
	sort.Slice(checkIns, func(i, j int) bool {
		return timelineStamp(checkIns[i]).Before(timelineStamp(checkIns[j]))
	})

	entries := make([]TimelineEntry, 0, len(checkIns)+1)

	prev := time.Time{}
	if !run.StartedAt.IsZero() {
		entries = append(entries, TimelineEntry{Kind: TimelineStart, At: run.StartedAt})
		prev = run.StartedAt
	}

	for _, checkIn := range checkIns {
		at := timelineStamp(checkIn)

		kind := TimelineDone
		if run.MustCheckOut != "" && checkIn.LocationID == run.MustCheckOut {
			kind = TimelineCurrent
		}

		entry := TimelineEntry{
			Kind:   kind,
			Label:  checkIn.Location.Name,
			At:     at,
			Points: checkIn.Points,
		}
		if !prev.IsZero() && at.After(prev) {
			entry.GapBefore = at.Sub(prev)
		}
		entries = append(entries, entry)
		prev = at
	}

	return entries
}

// TimeIn is unset on rows created before it existed; CreatedAt is the fallback.
func timelineStamp(checkIn models.CheckIn) time.Time {
	if !checkIn.TimeIn.IsZero() {
		return checkIn.TimeIn
	}
	return checkIn.CreatedAt
}

func runElapsed(run models.Run) string {
	if run.StartedAt.IsZero() {
		return ""
	}
	return formatDuration(time.Since(run.StartedAt))
}

// Scans for the maximum rather than indexing, so it survives an ordering change.
func runLastSeen(run models.Run) (time.Time, bool) {
	var latest time.Time
	for _, checkIn := range run.CheckIns {
		if at := timelineStamp(checkIn); at.After(latest) {
			latest = at
		}
	}
	return latest, !latest.IsZero()
}

func countNotStarted(runs []models.Run) int {
	count := 0
	for _, run := range runs {
		if !run.HasStarted {
			count++
		}
	}
	return count
}

func countFinished(runs []models.Run, totalObjectives int) int {
	if totalObjectives == 0 {
		return 0
	}
	count := 0
	for _, run := range runs {
		if len(run.CheckIns) >= totalObjectives {
			count++
		}
	}
	return count
}

// Median, not mean: dormant runs drag an average down and mask a healthy event.
func medianProgress(runs []models.Run) int {
	progress := make([]int, 0, len(runs))
	for _, run := range runs {
		if run.HasStarted {
			progress = append(progress, len(run.CheckIns))
		}
	}
	if len(progress) == 0 {
		return 0
	}
	sort.Ints(progress)

	mid := len(progress) / 2
	if len(progress)%2 == 1 {
		return progress[mid]
	}
	return (progress[mid-1] + progress[mid]) / 2
}

func countUnread(notifications []models.Notification) int {
	count := 0
	for _, notification := range notifications {
		if !notification.Dismissed {
			count++
		}
	}
	return count
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}
