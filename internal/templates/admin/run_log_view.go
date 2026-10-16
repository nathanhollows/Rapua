package templates

import (
	"fmt"
	"slices"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
)

// progressKinds are the work done; views and the start count as navigation.
var progressKinds = []services.RunLogKind{
	services.RunLogAnswer,
	services.RunLogComplete,
	services.RunLogSection,
	services.RunLogUpload,
	services.RunLogFinish,
}

var messageKinds = []services.RunLogKind{
	services.RunLogMessage,
	services.RunLogRead,
}

var navigationKinds = []services.RunLogKind{
	services.RunLogViewed,
	services.RunLogListed,
}

// runLogFilter shows everything for an unknown filter rather than nothing.
func runLogFilter(entries []services.RunLogEntry, show string) []services.RunLogEntry {
	var kinds []services.RunLogKind
	switch show {
	case "progress":
		kinds = progressKinds
	case "messages":
		kinds = messageKinds
	case "navigation":
		kinds = navigationKinds
	default:
		return entries
	}
	shown := make([]services.RunLogEntry, 0, len(entries))
	for _, entry := range entries {
		if slices.Contains(kinds, entry.Kind) {
			shown = append(shown, entry)
		}
	}
	return shown
}

// RunLogCardTarget is the id of the timeline card and the Hx-Target its filters
// send. RunOverview renders the card alone for that request, so the markup and
// the handler have to agree on it.
const RunLogCardTarget = "run-log-card"

func runLogHref(runCode, show string) string {
	if show == "" {
		return "/admin/runs/" + runCode
	}
	return "/admin/runs/" + runCode + "?show=" + show
}

func runLogFooter(shown, total int) string {
	if shown == total {
		return fmt.Sprintf("%d entries, the whole run.", total)
	}
	return fmt.Sprintf("%d of %d entries, the whole run.", shown, total)
}

// runStatus reads the tail of the strip; a run with nothing to read is just started.
func runStatus(run models.Run, log services.RunLog) services.TeamStatus {
	if log.Finished {
		return services.StatusFinished
	}
	if !run.HasStarted {
		return services.StatusNotStarted
	}
	if len(log.Segments) == 0 {
		return services.StatusStarted
	}
	return log.Segments[len(log.Segments)-1].Class.Status()
}

func runLogQuietLine(entry services.RunLogEntry) string {
	return fmt.Sprintf("%s for %s", cadenceName(entry.Cadence), formatDuration(entry.QuietFor))
}

// runLogQuietTint matches the strip's band colours.
func runLogQuietTint(class services.CadenceClass) string {
	switch class {
	case services.CadenceIdle:
		return "text-info"
	case services.CadenceStalled:
		return "text-warning"
	case services.CadenceAbandoned:
		return "text-error"
	case services.CadencePlaying:
		// Playing earns no quiet line to tint, and naming it here is what makes
		// a new class a compile-time question rather than a silent default.
		return ""
	}
	return ""
}

func cadenceName(class services.CadenceClass) string {
	switch class {
	case services.CadenceIdle:
		return "Idle"
	case services.CadenceStalled:
		return "Stalled"
	case services.CadenceAbandoned:
		return "Abandoned"
	case services.CadencePlaying:
		return "Playing"
	}
	return "Quiet"
}

// runLogQuiet exists because the strip's capped tail no longer shows the silence.
func runLogQuiet(log services.RunLog) string {
	if log.Quiet <= 0 {
		return ""
	}
	return fmt.Sprintf("Quiet for %s", formatDuration(log.Quiet))
}

func runLogPace(log services.RunLog) string {
	if log.Pace <= 0 {
		return "—"
	}
	return formatDuration(log.Pace)
}

func runLogPaceNote(log services.RunLog) string {
	if log.QuestPace <= 0 {
		return "Between actions"
	}
	return fmt.Sprintf("Between actions · quest avg %s", formatDuration(log.QuestPace))
}

// runLogTime adds the day once the run crosses midnight, and the date once it
// crosses a Monday, because a bare clock or weekday stops being unambiguous.
func runLogTime(at time.Time, log services.RunLog) string {
	first, last := log.From.Local(), log.To.Local()
	at = at.Local()
	switch {
	case !sameWeek(first, last):
		return at.Format("2 Jan 3:04 PM")
	case first.YearDay() != last.YearDay():
		return at.Format("Mon 3:04 PM")
	default:
		return at.Format("3:04 PM")
	}
}

func sameWeek(a, b time.Time) bool {
	return mondayOf(a).Equal(mondayOf(b))
}

// mondayOf shifts because time.Weekday counts from Sunday.
func mondayOf(at time.Time) time.Time {
	offset := (int(at.Weekday()) + 6) % 7 //nolint:mnd // A week is seven days; Sunday counts as the sixth.
	day := at.AddDate(0, 0, -offset)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
}

func runLogWait(gap time.Duration) string {
	if gap <= 0 {
		return "—"
	}
	return formatDuration(gap)
}

// runLogSpan is a row's interval on the axis, in seconds. A quiet line spans
// the quiet itself, so the window covers the stretches between events.
//
// Both ends clamp independently: deriving the start from the clamped end would
// make the quiet line, whose foot is now, claim the whole strip.
func runLogSpan(entry services.RunLogEntry, log services.RunLog) (int, int) {
	wait := entry.Gap
	if entry.Kind == services.RunLogQuiet {
		wait = entry.QuietFor
	}

	axis := runLogAxis(log)
	end := clamp(int(entry.Time.Sub(log.From).Seconds()), 0, axis)
	start := clamp(int(entry.Time.Add(-wait).Sub(log.From).Seconds()), 0, end)
	return start, end
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// runLogAxis is never zero, so callers can divide by it.
func runLogAxis(log services.RunLog) int {
	seconds := int(log.To.Sub(log.From).Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

const percentScale = 100

func runLogSegmentPercent(segment services.CadenceSegment, log services.RunLog) float64 {
	span := log.To.Sub(log.From)
	if span <= 0 {
		return percentScale
	}
	return float64(segment.Duration) / float64(span) * percentScale
}

func runLogSegmentTitle(segment services.CadenceSegment) string {
	return fmt.Sprintf("%s, %s", segment.Class, formatDuration(segment.Duration))
}

// runLogUploadLabel switches on the upload's kind (image or video), not a MIME type.
func runLogUploadLabel(mediaType string) string {
	if mediaType == string(models.MediaTypeImage) {
		return "Uploaded a photo"
	}
	return "Uploaded a video"
}

func runLogIsImage(mediaType string) bool {
	return mediaType == string(models.MediaTypeImage)
}
