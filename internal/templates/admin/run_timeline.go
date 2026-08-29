package templates

import (
	"fmt"
	"sort"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
)

func runElapsed(run models.Run) string {
	if run.StartedAt.IsZero() {
		return ""
	}
	return formatDuration(time.Since(run.StartedAt))
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

// countFinished counts runs that have completed every objective. completedCounts
// is keyed by run code (see RunService.CountCompletedObjectivesByRun).
func countFinished(runs []models.Run, totalObjectives int, completedCounts map[string]int) int {
	if totalObjectives == 0 {
		return 0
	}
	count := 0
	for _, run := range runs {
		if completedCounts[run.Code] >= totalObjectives {
			count++
		}
	}
	return count
}

// Median, not mean: dormant runs drag an average down and mask a healthy event.
func medianProgress(runs []models.Run, completedCounts map[string]int) int {
	progress := make([]int, 0, len(runs))
	for _, run := range runs {
		if run.HasStarted {
			progress = append(progress, completedCounts[run.Code])
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
