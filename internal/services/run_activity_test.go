package services_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func teamEvent(run string, event models.RunEvent) models.RunEvent {
	event.RunCode = run
	return event
}

func solved(run string, minute int, points int) models.RunEvent {
	event := teamEvent(run, ev(at(minute), models.EventBlockAnswered, models.BlockAnsweredData{BlockType: "scan", Completed: true}))
	event.ObjectiveID = "o1"
	event.Points = points
	return event
}

func TestBuildActivity_NewestFirstWithTheTeamName(t *testing.T) {
	events := []models.RunEvent{
		teamEvent("AAAA", ev(at(1), models.EventRunStarted, nil)),
		solved("AAAA", 2, 30),
		solved("BBBB", 3, 10),
	}

	feed := services.BuildActivity(events, journalTree(), map[string]string{"AAAA": "Haka Hustlers"}, 10)

	require.Len(t, feed, 3)
	assert.Equal(t, "BBBB", feed[0].RunCode)
	assert.Empty(t, feed[0].RunName, "a team with no name has none to show")
	assert.Equal(t, "Haka Hustlers", feed[1].RunName)
	assert.Equal(t, 30, feed[1].Points)
	assert.Equal(t, "The library", feed[1].Title)
	assert.Equal(t, services.RunLogStart, feed[2].Kind)
}

// A misread QR is not an achievement, and a reveal's marker repeats the answer
// that cleared the proof, so neither is a line.
func TestBuildActivity_LeavesOutNoiseAndMisses(t *testing.T) {
	missed := teamEvent("AAAA", ev(at(1), models.EventBlockAnswered, models.BlockAnsweredData{BlockType: "scan"}))
	viewed := teamEvent("AAAA", ev(at(2), models.EventObjectiveViewed, nil))
	listed := teamEvent("AAAA", ev(at(3), models.EventObjectivesListed, nil))
	sent := teamEvent("AAAA", ev(at(4), models.EventNotificationSent, models.NotificationData{Content: "hi"}))

	feed := services.BuildActivity([]models.RunEvent{missed, viewed, listed, sent, solved("AAAA", 5, 10)}, journalTree(), nil, 10)

	require.Len(t, feed, 1)
	assert.Equal(t, services.RunLogAnswer, feed[0].Kind)
}

func TestBuildActivity_KeepsTheNewestWithinTheLimit(t *testing.T) {
	events := []models.RunEvent{solved("AAAA", 1, 1), solved("AAAA", 2, 2), solved("AAAA", 3, 3)}

	feed := services.BuildActivity(events, journalTree(), nil, 2)

	require.Len(t, feed, 2)
	assert.Equal(t, 3, feed[0].Points)
	assert.Equal(t, 2, feed[1].Points)
}

func TestBuildActivity_FinishAndUploadAreLines(t *testing.T) {
	upload := teamEvent("AAAA", ev(at(1), models.EventUploadAdded, models.UploadAddedData{URL: "/u/x.jpg", MediaType: "image"}))
	finish := teamEvent("AAAA", ev(at(2), models.EventRunCompleted, nil))

	feed := services.BuildActivity([]models.RunEvent{upload, finish}, journalTree(), nil, 10)

	require.Len(t, feed, 2)
	assert.Equal(t, services.RunLogFinish, feed[0].Kind)
	assert.Equal(t, services.RunLogUpload, feed[1].Kind)
}
