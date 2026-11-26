package services_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin's log keeps the whole stream, newest first: the start, the name,
// the work, the messages and the finish alike.
func TestBuildRunLog_KeepsTheWholeStream(t *testing.T) {
	named := models.RunEvent{Type: models.EventRunNamed, CreatedAt: at(2)}
	named.Data, _ = json.Marshal(models.RunNamedData{Name: "Leeroy Jenkins"})

	answered := models.RunEvent{
		Type: models.EventBlockAnswered, BlockID: "b1", ObjectiveID: "o1", CreatedAt: at(5),
	}
	answered.Data, _ = json.Marshal(models.BlockAnsweredData{BlockType: "quiz", Completed: true})
	answered.Points = 10

	viewed := models.RunEvent{Type: models.EventObjectiveViewed, ObjectiveID: "o1", CreatedAt: at(4)}

	complete := models.RunEvent{
		Type: models.EventObjectiveContextCompleted, ObjectiveID: "o1", CreatedAt: at(5),
	}
	complete.Data, _ = json.Marshal(
		models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof, Points: 10},
	)

	reveal := models.RunEvent{
		Type: models.EventObjectiveContextCompleted, ObjectiveID: "o1", CreatedAt: at(6),
	}
	reveal.Data, _ = json.Marshal(
		models.ObjectiveContextCompletedData{Context: game.ContextObjectiveReveal},
	)

	events := []models.RunEvent{
		ev(at(1), models.EventRunStarted, nil),
		named,
		ev(at(3), models.EventObjectivesListed, nil),
		viewed,
		complete,
		answered,
		reveal,
		ev(at(8), models.EventRunCompleted, nil),
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(8))

	// The reveal's completion marks the same moment as the view that triggers
	// it, so it is the one row the log does not draw.
	require.Len(t, log.Entries, 7)
	kinds := make([]services.RunLogKind, 0, len(log.Entries))
	for _, entry := range log.Entries {
		kinds = append(kinds, entry.Kind)
	}
	assert.Equal(t, []services.RunLogKind{
		services.RunLogFinish,
		services.RunLogAnswer,
		services.RunLogComplete,
		services.RunLogViewed,
		services.RunLogListed,
		services.RunLogNamed,
		services.RunLogStart,
	}, kinds, "newest first")

	answer := log.Entries[1]
	assert.Equal(t, "Quiz", answer.BlockName, "the payload's type names the block")
	assert.Equal(t, 10, answer.Points)
	assert.Equal(t, 1, answer.Tries)
	assert.True(t, answer.Completed)
	assert.Equal(t, "The library", answer.Title, "a block's row names its objective")
	assert.Equal(t, "the-library", answer.Slug)

	assert.Equal(t, "The library", log.Entries[2].Title)
	assert.Equal(t, "Leeroy Jenkins", log.Entries[5].Title)

	assert.Equal(t, 2*time.Minute, log.Entries[0].Gap,
		"the wait since the previous event, the rowless reveal included")
	assert.Zero(t, log.Entries[6].Gap, "nothing came before the start")
}

// A row the log has no shape for draws nothing. Streams written by older
// versions carry event types this build does not know, and a blank row would
// claim the log has something to say about them.
func TestBuildRunLog_IgnoresWhatItCannotDraw(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventRunStarted, nil),
		{Type: models.EventType("something_older"), CreatedAt: at(5)},
		ev(at(6), models.EventObjectivesListed, nil),
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(7))

	require.Len(t, log.Entries, 2, "the unknown row is not drawn")
	assert.Equal(t, services.RunLogListed, log.Entries[0].Kind)
	assert.Equal(t, services.RunLogStart, log.Entries[1].Kind)
	assert.Equal(t, at(1), log.From, "and the axis still starts where the run did")
}

// A finished run stops at its finish: nothing after it can be a walk-off,
// because there is no run left to walk off from.
func TestBuildRunLog_FinishedRunsStopAtTheFinish(t *testing.T) {
	message := models.RunEvent{Type: models.EventNotificationSent, CreatedAt: at(70)}
	message.Data, _ = json.Marshal(models.NotificationData{NotificationID: "n1", Content: "All done!"})

	events := []models.RunEvent{
		ev(at(0), models.EventRunStarted, nil),
		ev(at(1), models.EventObjectivesListed, nil),
		ev(at(2), models.EventObjectiveViewed, nil),
		ev(at(3), models.EventObjectivesListed, nil),
		ev(at(4), models.EventObjectiveViewed, nil),
		ev(at(10), models.EventRunCompleted, nil),
		message,
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(100))

	assert.True(t, log.Finished)
	assert.Equal(t, at(10), log.To, "the axis ends at the finish, not at now")
	for _, segment := range log.Segments {
		assert.NotEqual(t, services.CadenceAbandoned, segment.Class,
			"an hour of quiet after the finish is not an abandoned run")
	}
	// Seven events, plus the line for the six quiet minutes into the finish:
	// they came back for it, so it reads idle, never abandoned.
	require.Len(t, log.Entries, 8)
	assert.Equal(t, "All done!", log.Entries[0].Message)
	assert.Equal(t, services.RunLogQuiet, log.Entries[2].Kind)
	assert.Equal(t, services.CadenceIdle, log.Entries[2].Cadence)
}

// Streams written before presence stopped being recorded after a finish carry
// views past it, which no window on the axis can reach.
func TestBuildRunLog_DropsViewsPastTheFinish(t *testing.T) {
	events := []models.RunEvent{
		ev(at(0), models.EventRunStarted, nil),
		ev(at(2), models.EventObjectiveViewed, nil),
		ev(at(10), models.EventRunCompleted, nil),
		ev(at(20), models.EventObjectivesListed, nil),
		ev(at(21), models.EventObjectiveViewed, nil),
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(100))

	for _, entry := range log.Entries {
		assert.False(t, entry.Time.After(at(10)), "nothing drawn sits past the axis")
	}
}

// Tries are counted from the stream, per block, up to and including the row.
func TestBuildRunLog_CountsTriesPerBlock(t *testing.T) {
	var events []models.RunEvent
	for i, minute := range []int{1, 2, 3} {
		event := ev(at(minute), models.EventBlockAnswered,
			models.BlockAnsweredData{BlockType: "quiz", Completed: i == 2})
		event.BlockID = "b1"
		events = append(events, event)
	}
	other := ev(at(4), models.EventBlockAnswered, models.BlockAnsweredData{BlockType: "pincode"})
	other.BlockID = "b2"
	events = append(events, other)

	log := services.BuildRunLog(events, journalTree(), 0, at(4))

	require.Len(t, log.Entries, 4)
	assert.Equal(t, 1, log.Entries[0].Tries, "a different block counts from one")
	assert.Equal(t, 3, log.Entries[1].Tries)
	assert.Equal(t, 2, log.Entries[2].Tries)
	assert.Equal(t, 1, log.Entries[3].Tries)
}

// A wait is read against the run's own pace: under two times playing, then
// idle, then stalled, and abandoned past eight. The class floors lift each
// threshold, so a one-minute pace still needs five minutes of quiet for idle.
// Adjacent waits of one class merge into a band.
func TestBuildRunLog_ClassifiesWaitsAgainstTheRunPace(t *testing.T) {
	// Five one-minute waits set the pace at a minute (the floor); the
	// stragglers then land against the floors, and the tail is the current
	// quiet.
	minutes := []int{0, 1, 2, 3, 4, 5, 8, 13, 23}
	events := make([]models.RunEvent, 0, len(minutes))
	for _, minute := range minutes {
		events = append(events, ev(at(minute), models.EventObjectivesListed, nil))
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(24))

	assert.Equal(t, time.Minute, log.Baseline)
	require.Len(t, log.Segments, 4)
	assert.Equal(t, []services.CadenceSegment{
		{Class: services.CadencePlaying, Duration: 8 * time.Minute},
		{Class: services.CadenceIdle, Duration: 5 * time.Minute},
		{Class: services.CadenceStalled, Duration: 10 * time.Minute},
		{Class: services.CadencePlaying, Duration: time.Minute},
	}, log.Segments)
}

// A brisk run cannot sprout idle bands from waits that are short in absolute
// terms: five minutes of quiet is not idleness however fast the game.
func TestBuildRunLog_IdleKeepsAFlatFloor(t *testing.T) {
	// One-minute waits set a fast pace; a four-minute quiet is still play.
	events := []models.RunEvent{ev(at(0), models.EventRunStarted, nil)}
	for i := 1; i <= 5; i++ {
		events = append(events, ev(at(i), models.EventObjectivesListed, nil))
	}
	events = append(events, ev(at(9), models.EventObjectivesListed, nil))

	log := services.BuildRunLog(events, journalTree(), 0, at(9))

	assert.Equal(t, time.Minute, log.Baseline)
	require.Len(t, log.Segments, 1, "a four-minute wait is under the idle floor")
	assert.Equal(t, services.CadencePlaying, log.Segments[0].Class)
}

// Abandonment is only ever claimed of the quiet still running: a stretch the
// team came back from was a stall, however long, and the hour floor still
// applies to the open one.
func TestBuildRunLog_AbandonedKeepsAnHourFloorAndNeedsAnOpenQuiet(t *testing.T) {
	pace := func() []models.RunEvent {
		events := []models.RunEvent{ev(at(0), models.EventRunStarted, nil)}
		for minute := 1; minute <= 5; minute++ {
			events = append(events, ev(at(minute), models.EventObjectivesListed, nil))
		}
		return events
	}
	extend := func(events []models.RunEvent, last int) []models.RunEvent {
		return append(events, ev(at(last), models.EventObjectivesListed, nil))
	}

	// Thirty closed minutes: stalled.
	half := services.BuildRunLog(extend(pace(), 35), journalTree(), 0, at(36))
	require.Len(t, half.Segments, 3)
	assert.Equal(t, services.CadenceStalled, half.Segments[1].Class, "thirty minutes stalls")
	assert.Equal(t, 30*time.Minute, half.Segments[1].Duration)

	// Ninety closed minutes: still a stall, because they came back.
	returned := services.BuildRunLog(extend(pace(), 95), journalTree(), 0, at(96))
	require.Len(t, returned.Segments, 3)
	assert.Equal(t, services.CadenceStalled, returned.Segments[1].Class,
		"a stretch with events after it is a stall, not a walk-off, however long")

	// Ninety minutes and still nothing: abandoned.
	gone := services.BuildRunLog(pace(), journalTree(), 0, at(95))
	require.Len(t, gone.Segments, 2)
	assert.Equal(t, services.CadenceAbandoned, gone.Segments[1].Class,
		"the quiet still running is the one a walk-off can be read from")
}

// Quiet stretches long enough to read as idle or worse become lines in the
// timeline: the closed ones under the row that ended them, and the one still
// running at the top.
func TestBuildRunLog_MarksQuietStretches(t *testing.T) {
	events := []models.RunEvent{ev(at(0), models.EventRunStarted, nil)}
	for minute := 1; minute <= 5; minute++ {
		events = append(events, ev(at(minute), models.EventObjectivesListed, nil))
	}
	events = append(events,
		ev(at(35), models.EventObjectivesListed, nil),
		ev(at(40), models.EventObjectivesListed, nil),
	)

	log := services.BuildRunLog(events, journalTree(), 0, at(160))

	var lines []services.RunLogEntry
	for _, entry := range log.Entries {
		if entry.Kind == services.RunLogQuiet {
			lines = append(lines, entry)
		}
	}
	require.Len(t, lines, 3)

	// Newest first: the open two hours opens the feed.
	assert.Equal(t, services.CadenceAbandoned, lines[0].Cadence)
	assert.Equal(t, 2*time.Hour, lines[0].QuietFor)
	assert.Equal(t, services.RunLogQuiet, log.Entries[0].Kind, "the running quiet is the newest line")

	// The closed ones sit under the rows that ended them: the five-minute
	// wait as idle, the thirty-minute one as stalled.
	assert.Equal(t, services.CadenceIdle, lines[1].Cadence)
	assert.Equal(t, 5*time.Minute, lines[1].QuietFor)
	assert.Equal(t, services.CadenceStalled, lines[2].Cadence)
	assert.Equal(t, 30*time.Minute, lines[2].QuietFor)
}

// An abandoned run stops growing twice over: the logical cap makes an hour
// the longest quiet the class can mean, and the visual cap draws that quiet no
// wider than the play it followed, so five minutes of play before an abandoned
// afternoon still reads as five minutes of play and not a sliver. The true
// silence lives on the numbers.
func TestBuildRunLog_TheQuietTailIsCapped(t *testing.T) {
	events := []models.RunEvent{ev(at(0), models.EventRunStarted, nil)}
	for minute := 1; minute <= 5; minute++ {
		events = append(events, ev(at(minute), models.EventObjectivesListed, nil))
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(60*5))

	assert.Equal(t, 4*time.Hour+55*time.Minute, log.Quiet,
		"five hours since the first event, five minutes of it played")
	assert.Equal(t, at(5).Add(5*time.Minute), log.To,
		"the axis ends where the drawn tail does, at the width of the play")

	require.Len(t, log.Segments, 2)
	assert.Equal(t, services.CadencePlaying, log.Segments[0].Class)
	assert.Equal(t, 5*time.Minute, log.Segments[0].Duration)
	assert.Equal(t, services.CadenceAbandoned, log.Segments[1].Class,
		"an hour of quiet is still abandoned, however narrowly it is drawn")
	assert.Equal(t, 5*time.Minute, log.Segments[1].Duration)
}

// The tail's class reads the whole silence rather than the strip's capped
// width. On a pace slow enough that the cap falls short of eight times it, the
// quiet line says abandoned, and the strip must not say stalled beside it.
func TestBuildRunLog_SlowPaceTailReadsTheWholeSilence(t *testing.T) {
	var events []models.RunEvent
	for i := range 10 {
		events = append(events, ev(at(10*i), models.EventObjectivesListed, nil))
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(90).Add(2*time.Hour))

	assert.Equal(t, 10*time.Minute, log.Baseline)
	assert.Equal(t, 2*time.Hour, log.Quiet)

	require.Len(t, log.Segments, 2)
	assert.Equal(t, services.CadencePlaying, log.Segments[0].Class)
	assert.Equal(t, 90*time.Minute, log.Segments[0].Duration)
	assert.Equal(t, services.CadenceAbandoned, log.Segments[1].Class,
		"two hours of quiet on a ten-minute pace is a walk-off, capped strip or not")
	assert.Equal(t, time.Hour, log.Segments[1].Duration, "and the cap still governs the width")

	require.NotEmpty(t, log.Entries)
	assert.Equal(t, services.RunLogQuiet, log.Entries[0].Kind)
	assert.Equal(t, services.CadenceAbandoned, log.Entries[0].Cadence,
		"the line at the head of the feed and the band behind it agree")
}

// A run that has ended has nobody left to walk off: the wait before a finish
// is a stall however long it ran, because the team came back for the ending.
func TestBuildRunLog_FinishedTailIsNotAWalkOff(t *testing.T) {
	events := []models.RunEvent{ev(at(0), models.EventRunStarted, nil)}
	for minute := 1; minute <= 5; minute++ {
		events = append(events, ev(at(minute), models.EventObjectivesListed, nil))
	}
	events = append(events, ev(at(185), models.EventRunCompleted, nil))

	log := services.BuildRunLog(events, journalTree(), 0, at(200))

	assert.True(t, log.Finished)
	require.Len(t, log.Segments, 2)
	assert.Equal(t, services.CadenceStalled, log.Segments[1].Class,
		"three hours of quiet ending in the finish is a stall, not a walk-off")
	assert.Equal(t, at(185), log.To)
}

// A run too thin for its own pace borrows the quest's, and says so rather
// than claiming the quest's pace as its own.
func TestBuildRunLog_ThinRunBorrowsTheQuestPace(t *testing.T) {
	events := []models.RunEvent{
		ev(at(0), models.EventRunStarted, nil),
		ev(at(3), models.EventObjectivesListed, nil),
	}

	log := services.BuildRunLog(events, journalTree(), 6*time.Minute, at(4))

	assert.Equal(t, 6*time.Minute, log.Baseline)
	assert.Zero(t, log.Pace, "the run has no pace of its own to show")
	assert.Equal(t, 6*time.Minute, log.QuestPace)
	require.Len(t, log.Segments, 1, "three minutes and the tail both read as playing")
	assert.Equal(t, services.CadencePlaying, log.Segments[0].Class)
	assert.Equal(t, 4*time.Minute, log.Segments[0].Duration)
}

// With neither a run pace nor a quest one there is no verdict and no strip.
func TestBuildRunLog_NoPaceNoCadence(t *testing.T) {
	events := []models.RunEvent{ev(at(0), models.EventRunStarted, nil)}

	log := services.BuildRunLog(events, journalTree(), 0, at(5))

	assert.Zero(t, log.Baseline)
	assert.Empty(t, log.Segments)
	require.Len(t, log.Entries, 1)
	assert.Equal(t, at(5), log.To)
}

// A burst of quick events sets a three-second pace, which the floor lifts to a
// minute: ordinary play must not read as stalling.
func TestBuildRunLog_FloorKeepsQuickBurstsPlaying(t *testing.T) {
	var events []models.RunEvent
	for i := range 6 {
		events = append(events, ev(at(0).Add(time.Duration(i)*3*time.Second), models.EventObjectivesListed, nil))
	}

	log := services.BuildRunLog(events, journalTree(), 0, at(0).Add(18*time.Second+10*time.Second))

	assert.Equal(t, time.Minute, log.Baseline)
	require.Len(t, log.Segments, 1)
	assert.Equal(t, services.CadencePlaying, log.Segments[0].Class)
}

func TestBuildRunLog_EmptyStream(t *testing.T) {
	log := services.BuildRunLog(nil, journalTree(), 0, at(0))
	assert.Empty(t, log.Entries)
	assert.Empty(t, log.Segments)
	assert.Zero(t, log.Baseline)
}

// The note words an unsolved attempt's input for the reports that read without
// their block. A quiz's options and a sorting block's order are ids, so the
// quiz reads as a count and the sorting not at all.
func TestBuildRunLog_WordsWhatAnUnsolvedAttemptSent(t *testing.T) {
	cases := []struct {
		name      string
		blockType string
		submitted any
		completed bool
		want      string
	}{
		{"password", "password", map[string]string{"answer": "hunter2"}, false, `"hunter2"`},
		{"pincode", "pincode", map[string]string{"pincode": "1234"}, false, `"1234"`},
		{
			"scan", "scan",
			map[string]any{"scanned": "ABC-123", "modality": "typed"}, false,
			`"ABC-123" (typed)`,
		},
		{
			"scan with no modality", "scan",
			map[string]any{"scanned": "ABC-123"}, false,
			`"ABC-123"`,
		},
		{
			"geofence", "geofence",
			map[string]float64{"metres_away": 140.4}, false,
			"140 metres away",
		},
		{
			"checklist", "checklist",
			map[string][]string{"checked_items": {"a", "b", "c"}}, false,
			"3 ticked",
		},
		{
			"quiz reports a count", "quiz",
			map[string][]string{"selected_options": {"option_0", "option_2"}}, false,
			"2 selected",
		},
		{
			"a wrong answer that closed its block", "quiz",
			map[string][]string{"selected_options": {"option_0"}}, true,
			"1 selected",
		},
		{
			"sorting is ids, so nothing", "sorting",
			map[string][]string{"player_order": {"i1", "i2"}}, false, "",
		},
		{"a block that never reports", "free_text", nil, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answered := ev(at(2), models.EventBlockAnswered, models.BlockAnsweredData{
				BlockType: tc.blockType, Completed: tc.completed,
			})
			if tc.submitted != nil {
				data, err := json.Marshal(tc.submitted)
				require.NoError(t, err)
				var payload models.BlockAnsweredData
				require.NoError(t, json.Unmarshal(answered.Data, &payload))
				payload.Submitted = data
				answered = ev(at(2), models.EventBlockAnswered, payload)
			}

			log := services.BuildRunLog(
				[]models.RunEvent{ev(at(1), models.EventRunStarted, nil), answered},
				journalTree(), 0, at(3),
			)

			require.NotEmpty(t, log.Entries)
			assert.Equal(t, tc.want, log.Entries[0].Submitted)
			assert.Equal(t, tc.completed, log.Entries[0].Completed,
				"the note is about the attempt, whether or not the block closed")
		})
	}
}

// The quest's pace pools every run's own waits.
func TestQuestBaseline(t *testing.T) {
	times := map[string][]time.Time{
		"a": {at(0), at(2), at(4)},
		"b": {at(0), at(6)},
	}
	assert.Equal(t, 2*time.Minute, services.QuestBaseline(times), "gaps 2m, 2m and 6m")
	assert.Zero(t, services.QuestBaseline(nil))
	assert.Zero(t, services.QuestBaseline(map[string][]time.Time{"a": {at(0)}}))
}
