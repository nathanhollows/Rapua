package services

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// RunLogKind names what one line of the admin's run log is.
type RunLogKind string

const (
	RunLogStart  RunLogKind = "start"
	RunLogNamed  RunLogKind = "named"
	RunLogListed RunLogKind = "listed"
	RunLogViewed RunLogKind = "viewed"
	// RunLogAnswer is never right or wrong: the stream records completion and
	// worth, and only the block knows what correct means.
	RunLogAnswer   RunLogKind = "answer"
	RunLogComplete RunLogKind = "complete"
	RunLogSection  RunLogKind = "section"
	RunLogUpload   RunLogKind = "upload"
	RunLogMessage  RunLogKind = "message"
	RunLogRead     RunLogKind = "read"
	RunLogFinish   RunLogKind = "finish"
	// RunLogQuiet is the absence of an event. Only idle or worse earns a line;
	// ordinary gaps are just the wait beside a row.
	RunLogQuiet RunLogKind = "quiet"
)

// CadenceClass names how a stretch of the run reads against the run's own pace.
type CadenceClass string

const (
	CadencePlaying   CadenceClass = "playing"
	CadenceIdle      CadenceClass = "idle"
	CadenceStalled   CadenceClass = "stalled"
	CadenceAbandoned CadenceClass = "abandoned"
)

// Status is the class as a badge. The strip's bands and the lists' words are
// one fact told twice, so the mapping lives here rather than in each reader.
func (c CadenceClass) Status() TeamStatus {
	switch c {
	case CadencePlaying:
		return StatusPlaying
	case CadenceIdle:
		return StatusIdle
	case CadenceStalled:
		return StatusStalled
	case CadenceAbandoned:
		return StatusAbandoned
	}
	return StatusStarted
}

const (
	// Multiples of the run's own pace.
	cadenceIdleAt      = 2
	cadenceStalledAt   = 4
	cadenceAbandonedAt = 8
	// cadenceMinBaseline stops a burst of quick events making ordinary play
	// read as stalling.
	cadenceMinBaseline = time.Minute
	// Flat floors per class, so a brisk run cannot sprout idle bands from waits
	// that are short in absolute terms. An hour is the shortest quiet that
	// reads as a walk-off rather than lunch.
	cadenceMinIdle      = 5 * time.Minute
	cadenceMinStalled   = 10 * time.Minute
	cadenceMinAbandoned = time.Hour
	// cadenceMinGaps is the sample below which the quest's pace stands in.
	cadenceMinGaps = 4
)

// meaningfulTypes are the events that count as the team doing something.
// Admin-driven ones (notifications) would pollute the pace.
//
//nolint:gochecknoglobals // A definition, not state: one place says what counts.
var meaningfulTypes = []models.EventType{
	models.EventRunStarted,
	models.EventObjectivesListed,
	models.EventObjectiveViewed,
	models.EventBlockAnswered,
	models.EventObjectiveContextCompleted,
	models.EventSectionFinished,
	models.EventUploadAdded,
	models.EventRunCompleted,
}

// RunLogEntry is one line of the admin's run log.
type RunLogEntry struct {
	Kind      RunLogKind
	Time      time.Time
	Title     string
	Slug      string
	BlockName string
	// Completed means the submission closed its block, not that it was right.
	Completed bool
	// Points is as ledgered: the credited delta, negative for a block that
	// deducts.
	Points int
	// Tries counts submissions to this block up to and including this one.
	Tries   int
	Message string
	// MediaURL may outlive the file.
	MediaURL  string
	MediaType string
	// Gap is the wait since the previous entry of any kind.
	Gap      time.Duration
	Cadence  CadenceClass
	QuietFor time.Duration
}

// CadenceSegment is a stretch of the timeline in one class, neighbours merged.
type CadenceSegment struct {
	Class    CadenceClass
	Duration time.Duration
}

// RunLog is the admin's view of one run, newest first.
type RunLog struct {
	Entries []RunLogEntry
	// Segments is the strip, oldest first.
	Segments []CadenceSegment
	// From and To bound the strip's axis. A quiet tail is capped so a walk-off
	// cannot shrink the play to a sliver.
	From time.Time
	To   time.Time
	// Quiet keeps a long silence's true length, which the capped strip loses.
	Quiet time.Duration
	// Baseline is Pace, or QuestPace while the run is too thin to have one.
	// Zero means there is no cadence to draw.
	Baseline  time.Duration
	Pace      time.Duration
	QuestPace time.Duration
	Finished  bool
}

// BuildRunLog reads the run's whole stream. A finished run stops at its finish,
// because there is no run left to walk away from.
func BuildRunLog(
	events []models.RunEvent, objectives []models.Objective, questBaseline time.Duration, now time.Time,
) RunLog {
	if len(events) == 0 {
		return RunLog{QuestPace: questBaseline}
	}

	byID := make(map[string]models.Objective, len(objectives))
	for _, objective := range objectives {
		byID[objective.ID] = objective
	}
	blockNames := blockNamesByType()

	finishedAt := finishTime(events)
	times := meaningfulTimes(events, finishedAt)
	logicalTo := axisEnd(events, finishedAt, lastOf(times), now)

	pace := pacedMedian(gapsBetween(times))
	baseline := pace
	if baseline == 0 && questBaseline > 0 {
		baseline = max(questBaseline, cadenceMinBaseline)
	}
	// The tail is drawn capped but classed from the whole silence, so the strip
	// cannot disagree with the quiet line heading the feed.
	bands := capTail(
		cadenceBands(times, baseline, logicalTo, silenceEnd(finishedAt, now), finishedAt.IsZero()),
		playedOf(times),
	)

	// The axis follows the drawn end, which is shorter when the tail was capped.
	to := logicalTo
	if len(bands) > 0 {
		last := bands[len(bands)-1]
		to = last.Start.Add(last.Duration)
	}

	var quiet time.Duration
	if last := lastOf(times); finishedAt.IsZero() && !last.IsZero() && now.After(last) {
		quiet = now.Sub(last)
	}

	entries := runLogEntries(events, byID, blockNames, baseline)

	// The running quiet has no ending row, so its line opens the feed.
	if finishedAt.IsZero() && baseline > 0 && quiet > 0 {
		if class := cadenceClass(quiet, baseline, true); class != CadencePlaying {
			entries = append(entries, RunLogEntry{
				Kind:     RunLogQuiet,
				Time:     now,
				Cadence:  class,
				QuietFor: quiet,
			})
		}
	}

	slices.Reverse(entries)
	return RunLog{
		Entries:   entries,
		Segments:  mergeBands(bands),
		From:      events[0].CreatedAt,
		To:        to,
		Quiet:     quiet,
		Baseline:  baseline,
		Pace:      pace,
		QuestPace: questBaseline,
		Finished:  !finishedAt.IsZero(),
	}
}

// lastOf is zero when times is empty.
func lastOf(times []time.Time) time.Time {
	if len(times) == 0 {
		return time.Time{}
	}
	return times[len(times)-1]
}

// playedOf is the span from the first meaningful event to the last.
func playedOf(times []time.Time) time.Duration {
	if len(times) == 0 {
		return 0
	}
	first, last := times[0], times[len(times)-1]
	if !last.After(first) {
		return 0
	}
	return last.Sub(first)
}

// capTail draws the last band at most as wide as everything played, so an
// abandoned afternoon cannot squeeze a few minutes of play out of the strip.
func capTail(bands []cadenceBand, played time.Duration) []cadenceBand {
	if len(bands) == 0 {
		return bands
	}
	tail := bands[len(bands)-1]
	if tail.Duration <= played {
		return bands
	}
	if played <= 0 {
		// Nothing played, so nothing to weigh the tail against.
		return bands[:len(bands)-1]
	}
	bands[len(bands)-1].Duration = played
	return bands
}

// finishTime is zero while the run is unfinished.
func finishTime(events []models.RunEvent) time.Time {
	var finishedAt time.Time
	for i := range events {
		if events[i].Type == models.EventRunCompleted {
			finishedAt = events[i].CreatedAt
		}
	}
	return finishedAt
}

// axisEnd is the finish, or now with the quiet capped at the abandoned floor.
func axisEnd(events []models.RunEvent, finishedAt, lastMeaningful, now time.Time) time.Time {
	if !finishedAt.IsZero() {
		return finishedAt
	}
	if capped := lastMeaningful.Add(cadenceMinAbandoned); !lastMeaningful.IsZero() && now.After(capped) {
		now = capped
	}
	if last := events[len(events)-1].CreatedAt; now.Before(last) {
		// Clock skew must not put the axis before its own events.
		return last
	}
	return now
}

// silenceEnd stops at the finish: a finished run has no quiet after it.
func silenceEnd(finishedAt, now time.Time) time.Time {
	if !finishedAt.IsZero() {
		return finishedAt
	}
	return now
}

// runLogEntries emits rows oldest first, with a quiet line wherever the wait
// reads as idle or worse.
func runLogEntries(
	events []models.RunEvent,
	byID map[string]models.Objective,
	blockNames map[string]string,
	baseline time.Duration,
) []RunLogEntry {
	tries := make(map[string]int)
	entries := make([]RunLogEntry, 0, len(events))
	var lastMeaningful time.Time
	for i := range events {
		event := events[i]
		meaningful := slices.Contains(meaningfulTypes, event.Type)

		// Before the row that ended it, so the reversed feed shows it beneath.
		if meaningful && baseline > 0 && !lastMeaningful.IsZero() {
			if quiet := event.CreatedAt.Sub(lastMeaningful); quiet > 0 {
				if class := cadenceClass(quiet, baseline, false); class != CadencePlaying {
					entries = append(entries, RunLogEntry{
						Kind:     RunLogQuiet,
						Time:     event.CreatedAt,
						Cadence:  class,
						QuietFor: quiet,
					})
				}
			}
		}
		if meaningful {
			lastMeaningful = event.CreatedAt
		}

		var gap time.Duration
		if i > 0 {
			gap = event.CreatedAt.Sub(events[i-1].CreatedAt)
		}
		if entry, ok := runLogEntryFor(event, byID, blockNames, tries, gap); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

// QuestBaseline is the pace a run too young for its own borrows. Zero when the
// quest has nothing to read: an invented pace is worse than none.
func QuestBaseline(timesByRun map[string][]time.Time) time.Duration {
	var gaps []time.Duration
	for _, times := range timesByRun {
		for i := 1; i < len(times); i++ {
			gaps = append(gaps, times[i].Sub(times[i-1]))
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	return medianDuration(gaps)
}

// meaningfulTimes stops at until; a zero until reads the whole stream.
func meaningfulTimes(events []models.RunEvent, until time.Time) []time.Time {
	times := make([]time.Time, 0, len(events))
	for _, event := range events {
		if !until.IsZero() && event.CreatedAt.After(until) {
			break
		}
		if slices.Contains(meaningfulTypes, event.Type) {
			times = append(times, event.CreatedAt)
		}
	}
	return times
}

func gapsBetween(times []time.Time) []time.Duration {
	gaps := make([]time.Duration, 0, len(times))
	for i := 1; i < len(times); i++ {
		gaps = append(gaps, times[i].Sub(times[i-1]))
	}
	return gaps
}

// pacedMedian is zero while there are too few gaps to mean anything.
func pacedMedian(gaps []time.Duration) time.Duration {
	if len(gaps) < cadenceMinGaps {
		return 0
	}
	return max(medianDuration(gaps), cadenceMinBaseline)
}

// cadenceBand is one wait; the last runs to the axis' end, as the current quiet.
type cadenceBand struct {
	Start    time.Time
	Class    CadenceClass
	Duration time.Duration
}

// cadenceBands classifies each wait against the baseline. drawnTo and readTo
// part only for an open tail past the strip's cap, which is drawn at the cap
// but classed from the whole silence.
func cadenceBands(times []time.Time, baseline time.Duration, drawnTo, readTo time.Time, open bool) []cadenceBand {
	if baseline <= 0 || len(times) == 0 {
		return nil
	}
	bands := make([]cadenceBand, 0, len(times))
	for i, start := range times {
		end, quietEnd := drawnTo, readTo
		last := i+1 == len(times)
		if !last {
			end = times[i+1]
			quietEnd = end
		}
		if end.Before(start) {
			end = start
		}
		if end.Equal(start) {
			continue
		}
		if quietEnd.Before(start) {
			quietEnd = start
		}
		bands = append(bands, cadenceBand{
			Start:    start,
			Class:    cadenceClass(quietEnd.Sub(start), baseline, open && last),
			Duration: end.Sub(start),
		})
	}
	return bands
}

// cadenceClass reads one wait against the pace. Abandonment is claimed only of
// a wait still running: one the team came back from was a stall.
func cadenceClass(gap, baseline time.Duration, running bool) CadenceClass {
	class := CadenceAbandoned
	switch {
	case gap < max(cadenceIdleAt*baseline, cadenceMinIdle):
		class = CadencePlaying
	case gap < max(cadenceStalledAt*baseline, cadenceMinStalled):
		class = CadenceIdle
	case gap < max(cadenceAbandonedAt*baseline, cadenceMinAbandoned):
		class = CadenceStalled
	}
	if class == CadenceAbandoned && !running {
		return CadenceStalled
	}
	return class
}

// mergeBands folds neighbouring waits of one class into one segment.
func mergeBands(bands []cadenceBand) []CadenceSegment {
	if len(bands) == 0 {
		return nil
	}
	segments := make([]CadenceSegment, 0, len(bands))
	for _, band := range bands {
		if n := len(segments) - 1; n >= 0 && segments[n].Class == band.Class {
			segments[n].Duration += band.Duration
			continue
		}
		segments = append(segments, CadenceSegment{Class: band.Class, Duration: band.Duration})
	}
	return segments
}

// medianDuration is a median because one long gap, such as a lunch break,
// would inflate a mean and hide the stall it should flag.
func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	mid := len(sorted) / 2 //nolint:mnd // Finding the middle element, not a scale.
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2 //nolint:mnd // The mean of the two middles.
}

// runLogEntryFor draws a row for every event it has a case for: the stream is
// short, and a reader hunting for what happened does not want it pre-filtered.
func runLogEntryFor(
	event models.RunEvent,
	byID map[string]models.Objective,
	blockNames map[string]string,
	tries map[string]int,
	gap time.Duration,
) (RunLogEntry, bool) {
	entry := RunLogEntry{Time: event.CreatedAt, Gap: gap, Points: event.Points}

	switch event.Type {
	case models.EventRunStarted:
		entry.Kind = RunLogStart

	case models.EventRunNamed:
		var payload models.RunNamedData
		if json.Unmarshal(event.Data, &payload) != nil {
			return RunLogEntry{}, false
		}
		entry.Kind = RunLogNamed
		entry.Title = payload.Name

	case models.EventObjectivesListed:
		entry.Kind = RunLogListed

	case models.EventObjectiveViewed:
		entry.Kind = RunLogViewed
		objectiveInto(&entry, event, byID)

	case models.EventBlockAnswered:
		return runLogAnswerEntry(entry, event, byID, blockNames, tries)

	case models.EventObjectiveContextCompleted:
		var payload models.ObjectiveContextCompletedData
		if json.Unmarshal(event.Data, &payload) != nil {
			return RunLogEntry{}, false
		}
		if payload.Context != game.ContextObjectiveProof {
			// A reveal's marker repeats either the proof it followed or the
			// block answers that cleared it, and both already draw.
			return RunLogEntry{}, false
		}
		entry.Kind = RunLogComplete
		objectiveInto(&entry, event, byID)

	case models.EventSectionFinished:
		entry.Kind = RunLogSection
		objectiveInto(&entry, event, byID)

	case models.EventUploadAdded:
		var payload models.UploadAddedData
		if json.Unmarshal(event.Data, &payload) != nil {
			return RunLogEntry{}, false
		}
		entry.Kind = RunLogUpload
		entry.MediaURL = payload.URL
		entry.MediaType = payload.MediaType

	case models.EventNotificationSent:
		var payload models.NotificationData
		if json.Unmarshal(event.Data, &payload) != nil {
			return RunLogEntry{}, false
		}
		entry.Kind = RunLogMessage
		entry.Message = payload.Content

	case models.EventNotificationRead:
		entry.Kind = RunLogRead

	case models.EventRunCompleted:
		entry.Kind = RunLogFinish

	default:
		// An event the log has no row for draws nothing. Rendering it as an
		// empty row would put a blank line in the feed and say the log knows
		// something it does not.
		return RunLogEntry{}, false
	}

	return entry, true
}

// runLogAnswerEntry also counts tries, which only the stream order can say.
func runLogAnswerEntry(
	entry RunLogEntry,
	event models.RunEvent,
	byID map[string]models.Objective,
	blockNames map[string]string,
	tries map[string]int,
) (RunLogEntry, bool) {
	var payload models.BlockAnsweredData
	if json.Unmarshal(event.Data, &payload) != nil {
		return RunLogEntry{}, false
	}
	entry.Kind = RunLogAnswer
	entry.BlockName = blockNames[payload.BlockType]
	if entry.BlockName == "" {
		// An unregistered type still shows rather than vanishing.
		entry.BlockName = payload.BlockType
	}
	entry.Completed = payload.Completed
	if event.BlockID != "" {
		tries[event.BlockID]++
		entry.Tries = tries[event.BlockID]
	}
	objectiveInto(&entry, event, byID)
	return entry, true
}

func objectiveInto(entry *RunLogEntry, event models.RunEvent, byID map[string]models.Objective) {
	if event.ObjectiveID == "" {
		return
	}
	if objective, ok := byID[event.ObjectiveID]; ok {
		entry.Title = objective.Title
		entry.Slug = objective.Slug
	}
}

// blockNamesByType lets a line say "Quiz" from the payload's type string alone.
func blockNamesByType() map[string]string {
	names := make(map[string]string)
	for _, registration := range blocks.GetRegisteredBlocks() {
		names[registration.BlockType] = registration.Prototype.GetName()
	}
	return names
}
