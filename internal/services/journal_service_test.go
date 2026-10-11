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

// ev builds one recorded event with a marshalled payload.
func ev(at time.Time, typ models.EventType, payload any) models.RunEvent {
	event := models.RunEvent{Type: typ, CreatedAt: at}
	if payload != nil {
		data, _ := json.Marshal(payload)
		event.Data = data
	}
	return event
}

// journalTree is the fixture quest: a root objective and a section of three,
// which is enough for every resolution the journal does.
func journalTree() []models.Objective {
	return []models.Objective{
		{ID: "o1", Slug: "the-library", Title: "The library"},
		{ID: "section", Slug: "campus", Title: "Campus"},
		{ID: "c1", ParentID: "section", Slug: "c-one", Title: "Child one"},
		{ID: "c2", ParentID: "section", Slug: "c-two", Title: "Child two"},
		{ID: "c3", ParentID: "section", Slug: "c-three", Title: "Child three"},
	}
}

func at(minutes int) time.Time {
	return time.Date(2026, 10, 11, 18, 4, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

// Only the moments survive: views, listings, reads, and submissions that
// completed their block are all in the stream and none of them is an entry.
func TestBuildJournal_CurationKeepsOnlyTheMoments(t *testing.T) {
	find := models.RunEvent{
		Type: models.EventObjectiveContextCompleted, ObjectiveID: "o1", CreatedAt: at(4),
	}
	find.Data, _ = json.Marshal(
		models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof},
	)

	answer := ev(at(3), models.EventBlockAnswered,
		models.BlockAnsweredData{BlockType: "quiz", Completed: true, PointsAwarded: 10})
	answer.ObjectiveID = "o1"
	answer.Points = 10

	events := []models.RunEvent{
		ev(at(1), models.EventObjectiveViewed, nil),
		ev(at(2), models.EventObjectivesListed, nil),
		answer,
		find,
		ev(at(5), models.EventNotificationSent, models.NotificationData{NotificationID: "n1", Content: "Hello"}),
		ev(at(6), models.EventNotificationRead, models.NotificationData{NotificationID: "n1"}),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{})

	// The find and the message, newest first: the two moments in a stream of
	// six that a player would tell someone about.
	require.Len(t, view.Entries, 2)
	assert.Equal(t, services.JournalMessage, view.Entries[0].Kind)
	assert.Equal(t, services.JournalFind, view.Entries[1].Kind)
	assert.Equal(t, "The library", view.Entries[1].Title)
	assert.Equal(t, "the-library", view.Entries[1].Slug)
	assert.Equal(t, 10, view.Entries[1].Points)
}

// A find badges with what the objective had paid by the time it cleared, not
// with the closing submission's credit: the block that closes a proof may be
// worth nothing on its own, and the window shuts at the completion.
func TestBuildJournal_FindSumsWhatTheObjectivePaid(t *testing.T) {
	first := ev(at(1), models.EventBlockAnswered,
		models.BlockAnsweredData{BlockType: "quiz", Completed: true, PointsAwarded: 10})
	first.ObjectiveID = "o1"
	first.Points = 10

	closing := ev(at(2), models.EventBlockAnswered,
		models.BlockAnsweredData{BlockType: "text", Completed: true})
	closing.ObjectiveID = "o1"

	find := ev(at(3), models.EventObjectiveContextCompleted,
		models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof})
	find.ObjectiveID = "o1"

	// An answer landing after the clear is not the find's credit.
	late := ev(at(4), models.EventBlockAnswered,
		models.BlockAnsweredData{BlockType: "quiz", Completed: true, PointsAwarded: 5})
	late.ObjectiveID = "o1"
	late.Points = 5

	view := services.BuildJournal(
		[]models.RunEvent{first, closing, find, late}, journalTree(), map[string]bool{},
	)

	require.Len(t, view.Entries, 1)
	assert.Equal(t, services.JournalFind, view.Entries[0].Kind)
	assert.Equal(t, 10, view.Entries[0].Points)
}

// Entries run newest first, and the reset is the floor: nothing older than it
// is drawn, though the feed still ends on its line.
func TestBuildJournal_TheResetIsTheFloor(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventObjectiveContextCompleted,
			models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof, Points: 10}),
		ev(at(10), models.EventRunReset, nil),
		ev(at(20), models.EventObjectiveContextCompleted,
			models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof, Points: 10}),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{})

	require.Len(t, view.Entries, 2, "the pre-reset find is hidden")
	assert.Equal(t, services.JournalFind, view.Entries[0].Kind)
	assert.Equal(t, at(20), view.Entries[0].Time)
	assert.Equal(t, services.JournalReset, view.Entries[1].Kind)
	assert.Equal(t, at(10), view.Entries[1].Time)
}

// The opening line folds the greeting and the name into one, and takes the name
// as last set: the name is how the run is remembered.
func TestBuildJournal_OpeningLineTakesTheName(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventRunStarted, nil),
		ev(at(2), models.EventRunNamed, models.RunNamedData{Name: "Leeroy Jenkins"}),
		ev(at(3), models.EventRunNamed, models.RunNamedData{Name: "Leeroy Again"}),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{})

	require.Len(t, view.Entries, 1)
	assert.Equal(t, services.JournalStart, view.Entries[0].Kind)
	assert.Equal(t, "Leeroy Again", view.Entries[0].Title)
}

// A submission that did not complete its block is a not-yet; one that did is
// the moment the find already tells, so it is not repeated here.
func TestBuildJournal_MissesSayNotYet(t *testing.T) {
	miss := &models.RunEvent{
		Type: models.EventBlockAnswered, ObjectiveID: "o1", CreatedAt: at(1),
	}
	miss.Data, _ = json.Marshal(models.BlockAnsweredData{BlockType: "quiz"})
	completed := &models.RunEvent{
		Type: models.EventBlockAnswered, ObjectiveID: "o1", CreatedAt: at(2),
	}
	completed.Data, _ = json.Marshal(models.BlockAnsweredData{BlockType: "quiz", Completed: true})
	ownerless := &models.RunEvent{Type: models.EventBlockAnswered, CreatedAt: at(3)}
	ownerless.Data, _ = json.Marshal(models.BlockAnsweredData{BlockType: "password"})

	view := services.BuildJournal(
		[]models.RunEvent{*miss, *completed, *ownerless}, journalTree(), map[string]bool{},
	)

	require.Len(t, view.Entries, 2, "the completing submission is not a second line")
	assert.Equal(t, services.JournalMiss, view.Entries[0].Kind)
	assert.Empty(t, view.Entries[0].Title, "a quest-owned block has no objective to name")
	assert.Equal(t, "The library", view.Entries[1].Title)
}

// A read annotates its message with how long it waited; an unread one waits.
func TestBuildJournal_ReadsAnnotateTheirMessage(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventNotificationSent,
			models.NotificationData{NotificationID: "n1", Content: "Head to the quad"}),
		ev(at(2), models.EventNotificationRead, models.NotificationData{NotificationID: "n1"}),
		ev(at(3), models.EventNotificationSent, models.NotificationData{NotificationID: "n2", Content: "Last call"}),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{})

	require.Len(t, view.Entries, 2)
	assert.Equal(t, "Last call", view.Entries[0].Message)
	assert.False(t, view.Entries[0].Read)
	assert.Equal(t, "Head to the quad", view.Entries[1].Message)
	assert.True(t, view.Entries[1].Read)
	assert.Equal(t, time.Minute, view.Entries[1].ReadWait)
}

// A milestone counts what was found against the children in play, drafts
// excluded, and draws from the event's own time.
func TestBuildJournal_MilestoneCountsInPlayChildren(t *testing.T) {
	objectives := append(journalTree(), models.Objective{
		ID: "c4", ParentID: "section", Slug: "c-four", Title: "Child four", Draft: true,
	})
	events := []models.RunEvent{
		{Type: models.EventSectionFinished, ObjectiveID: "section", CreatedAt: at(9)},
	}

	view := services.BuildJournal(events, objectives, map[string]bool{"c1": true, "c2": true})

	require.Len(t, view.Entries, 1)
	entry := view.Entries[0]
	assert.Equal(t, services.JournalMilestone, entry.Kind)
	assert.Equal(t, "Campus", entry.Title)
	assert.Equal(t, 2, entry.Found)
	assert.Equal(t, 3, entry.Children, "the drafted child is not work anyone owes")
}

// The finish is the story's last line, and it sits at the top of a newest-first
// feed: the most recent thing that happened.
func TestBuildJournal_TheFinishIsTheLastLine(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventRunStarted, nil),
		ev(at(20), models.EventObjectiveContextCompleted,
			models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof, Points: 10}),
		ev(at(20), models.EventRunCompleted, nil),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{"o1": true})

	require.Len(t, view.Entries, 3)
	assert.Equal(t, services.JournalFinish, view.Entries[0].Kind)
	assert.Equal(t, at(20), view.Entries[0].Time)
}

// A reset hides everything before it, an earlier finish included: the leg the
// player is on has not been finished yet.
func TestBuildJournal_ResetHidesAnEarlierFinish(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventRunCompleted, nil),
		ev(at(5), models.EventRunReset, nil),
		ev(at(10), models.EventRunStarted, nil),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{})

	require.Len(t, view.Entries, 2)
	assert.Equal(t, services.JournalStart, view.Entries[0].Kind)
	assert.Equal(t, services.JournalReset, view.Entries[1].Kind)
}

func TestBuildJournal_EmptyStream(t *testing.T) {
	view := services.BuildJournal(nil, journalTree(), map[string]bool{})
	assert.Empty(t, view.Entries)
}

// A reveal completing is never an entry, whenever it lands: the content was on
// the page at the moment it completed, and the find's link is where the reward
// survives for later. Telling the player about it repeats what they just did.
func TestBuildJournal_RevealsAreNotEntries(t *testing.T) {
	events := []models.RunEvent{
		ev(at(1), models.EventObjectiveContextCompleted,
			models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof, Points: 10}),
		ev(at(1), models.EventObjectiveContextCompleted,
			models.ObjectiveContextCompletedData{Context: game.ContextObjectiveReveal}),
	}

	view := services.BuildJournal(events, journalTree(), map[string]bool{})

	require.Len(t, view.Entries, 1)
	assert.Equal(t, services.JournalFind, view.Entries[0].Kind)
}
