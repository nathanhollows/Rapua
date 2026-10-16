package services //nolint:testpackage // The entry builders are unexported.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every event type has to be decided by both surfaces, because both default to
// not drawn: a type added without a case is invisible in the admin's log and
// the player's diary, and nothing complains. The list comes from
// models.AllEventTypes, which sits beside the constants, so a new type fails
// this test until somebody writes down what it should do.
func TestEventTaxonomy_EveryTypeIsDecided(t *testing.T) {
	payload := func(value any) json.RawMessage {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		return data
	}
	const objectiveID = "o1"
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		event   models.RunEvent
		log     bool
		journal bool
	}{
		{
			name:    "run started",
			event:   models.RunEvent{Type: models.EventRunStarted, CreatedAt: at},
			log:     true,
			journal: true,
		},
		{
			// The name folds into the journal's opening line, so it is not a
			// line of its own there.
			name: "named",
			event: models.RunEvent{
				Type: models.EventRunNamed, CreatedAt: at,
				Data: payload(models.RunNamedData{Name: "Leeroy Jenkins"}),
			},
			log:     true,
			journal: false,
		},
		{
			name:    "listed",
			event:   models.RunEvent{Type: models.EventObjectivesListed, CreatedAt: at},
			log:     true,
			journal: false,
		},
		{
			name: "viewed",
			event: models.RunEvent{
				Type: models.EventObjectiveViewed, ObjectiveID: objectiveID, CreatedAt: at,
			},
			log:     true,
			journal: false,
		},
		{
			name: "answered, not completed",
			event: models.RunEvent{
				Type: models.EventBlockAnswered, ObjectiveID: objectiveID, BlockID: "b1", CreatedAt: at,
				Data: payload(models.BlockAnsweredData{BlockType: "quiz"}),
			},
			log:     true,
			journal: true,
		},
		{
			// The find is the diary's line for a completed block, so the
			// submission that closed it says nothing twice.
			name: "answered, completed",
			event: models.RunEvent{
				Type: models.EventBlockAnswered, ObjectiveID: objectiveID, BlockID: "b1", CreatedAt: at,
				Data: payload(models.BlockAnsweredData{BlockType: "quiz", Completed: true}),
			},
			log:     true,
			journal: false,
		},
		{
			name: "proof completed",
			event: models.RunEvent{
				Type: models.EventObjectiveContextCompleted, ObjectiveID: objectiveID, CreatedAt: at,
				Data: payload(models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof}),
			},
			log:     true,
			journal: true,
		},
		{
			// Reveals can hold interactive blocks, so their completion lands
			// whenever those do. The block answers that cleared it already
			// draw, and the content is on the page at the moment it closes.
			name: "reveal completed",
			event: models.RunEvent{
				Type: models.EventObjectiveContextCompleted, ObjectiveID: objectiveID, CreatedAt: at,
				Data: payload(models.ObjectiveContextCompletedData{Context: game.ContextObjectiveReveal}),
			},
			log:     false,
			journal: false,
		},
		{
			name: "section finished",
			event: models.RunEvent{
				Type: models.EventSectionFinished, ObjectiveID: objectiveID, CreatedAt: at,
			},
			log:     true,
			journal: true,
		},
		{
			name: "upload added",
			event: models.RunEvent{
				Type: models.EventUploadAdded, BlockID: "b1", CreatedAt: at,
				Data: payload(models.UploadAddedData{URL: "/uploads/a.jpg", MediaType: "image"}),
			},
			log:     true,
			journal: true,
		},
		{
			name: "notification sent",
			event: models.RunEvent{
				Type: models.EventNotificationSent, CreatedAt: at,
				Data: payload(models.NotificationData{NotificationID: "n1", Content: "Head to the quad"}),
			},
			log:     true,
			journal: true,
		},
		{
			// A read annotates the message it belongs to rather than drawing
			// its own line.
			name: "notification read",
			event: models.RunEvent{
				Type: models.EventNotificationRead, CreatedAt: at,
				Data: payload(models.NotificationData{NotificationID: "n1"}),
			},
			log:     true,
			journal: false,
		},
		{
			name:    "run completed",
			event:   models.RunEvent{Type: models.EventRunCompleted, CreatedAt: at},
			log:     true,
			journal: true,
		},
	}

	byID := map[string]models.Objective{
		objectiveID: {ID: objectiveID, Slug: "the-leaf", Title: "The leaf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, drawn := runLogEntryFor(tc.event, byID, map[string]string{}, map[string]int{}, time.Minute)
			assert.Equal(t, tc.log, drawn, "the admin's log")

			_, shown := journalEntryFor(
				tc.event, 0, byID, map[string]time.Time{}, map[string][]string{}, map[string]bool{},
			)
			assert.Equal(t, tc.journal, shown, "the player's diary")
		})
	}

	decided := make(map[models.EventType]bool, len(cases))
	for _, tc := range cases {
		decided[tc.event.Type] = true
	}
	for _, eventType := range models.AllEventTypes {
		assert.True(t, decided[eventType], "%s has no decision here", eventType)
	}
}
