package templates

import (
	"fmt"
	"sort"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
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

// countRunning counts the runs still playing: started, and not yet finished.
func countRunning(runs []models.Run, summaries map[string]services.RunSummary) int {
	count := 0
	for _, run := range runs {
		if run.HasStarted && summaries[run.Code].Status != services.StatusFinished {
			count++
		}
	}
	return count
}

// countFinished counts the runs whose own story ended, which is the run's
// completion rather than every objective proved: a branch the run cut is work
// it never owed.
func countFinished(runs []models.Run, summaries map[string]services.RunSummary) int {
	count := 0
	for _, run := range runs {
		if summaries[run.Code].Status == services.StatusFinished {
			count++
		}
	}
	return count
}

// medianShare is the middle started run's progress, as a percentage of what
// that run owes. A share, because two runs on one quest need not be the same
// length; a median rather than a mean, because one run still on its first
// objective should not read as the whole game stalling.
//
// A run that owes nothing has no share and sits the round out.
func medianShare(runs []models.Run, summaries map[string]services.RunSummary) int {
	shares := make([]int, 0, len(runs))
	for _, run := range runs {
		summary, known := summaries[run.Code]
		if !known || !run.HasStarted || summary.Total == 0 {
			continue
		}
		shares = append(shares, summary.Done*percentScale/summary.Total)
	}
	if len(shares) == 0 {
		return 0
	}
	sort.Ints(shares)

	mid := len(shares) / 2
	if len(shares)%2 == 1 {
		return shares[mid]
	}
	return (shares[mid-1] + shares[mid]) / 2
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
