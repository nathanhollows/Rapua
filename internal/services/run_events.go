package services

import (
	"encoding/json"
	"time"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Each event is a pure function of what happened, so these are package functions
// rather than methods on a type.

// eventPayload marshals a payload. Every payload is a struct of plain data, so a
// failure would be a programming error rather than a runtime condition, and the
// column is nullable.
func eventPayload(v any) json.RawMessage {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return encoded
}

func eventBase(runCode, questID string, eventType models.EventType) *models.RunEvent {
	return &models.RunEvent{
		RunCode:   runCode,
		QuestID:   questID,
		Type:      eventType,
		CreatedAt: time.Now().UTC(),
	}
}

// runResetEvent records an admin resetting a run. Earlier events stay, so this
// marks the boundary rather than erasing what came before it.
func runResetEvent(runCode, questID string) *models.RunEvent {
	return eventBase(runCode, questID, models.EventRunReset)
}

// objectivesListedEvent records a player opening the list of objectives.
func objectivesListedEvent(team *models.Run) *models.RunEvent {
	return eventBase(team.Code, team.QuestID, models.EventObjectivesListed)
}

// objectiveViewedEvent records a player opening an objective.
func objectiveViewedEvent(team *models.Run, objectiveID string) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventObjectiveViewed)
	event.ObjectiveID = objectiveID
	return event
}

// sectionFinishedEvent records a player choosing to end a section. A section is
// an objective, so the event carries one.
func sectionFinishedEvent(team *models.Run, objectiveID string) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventSectionFinished)
	event.ObjectiveID = objectiveID
	return event
}

// uploadAddedEvent records media a player uploaded.
func uploadAddedEvent(runCode, questID, blockID, url, mediaType string) *models.RunEvent {
	event := eventBase(runCode, questID, models.EventUploadAdded)
	event.BlockID = blockID
	event.Data = eventPayload(models.UploadAddedData{URL: url, MediaType: mediaType})
	return event
}

// notificationSentEvent records a message being sent to a run.
func notificationSentEvent(team *models.Run, notificationID string) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventNotificationSent)
	event.Data = eventPayload(models.NotificationData{NotificationID: notificationID})
	return event
}

// notificationReadEvent records a player dismissing a message, which is not the
// same as seeing it.
func notificationReadEvent(team *models.Run, notificationID string) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventNotificationRead)
	event.Data = eventPayload(models.NotificationData{NotificationID: notificationID})
	return event
}

// runNamedEvent records a player naming or renaming their run.
func runNamedEvent(team *models.Run, name string) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventRunNamed)
	event.Data = eventPayload(models.RunNamedData{Name: name})
	return event
}

// runStartedEvent records a run being marked started.
func runStartedEvent(team *models.Run) *models.RunEvent {
	return eventBase(team.Code, team.QuestID, models.EventRunStarted)
}

// answeredEvent records one graded answer, right or wrong.
//
// pointsAwarded is what the block reported, which the reachability gate may still
// decline to credit.
func answeredEvent(
	team *models.Run, block blocks.Block, objectiveID string, completed bool, pointsAwarded int,
) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventBlockAnswered)
	event.BlockID = block.GetID()
	event.ObjectiveID = objectiveID
	event.Data = eventPayload(models.BlockAnsweredData{
		BlockType:     block.GetType(),
		Completed:     completed,
		PointsAwarded: pointsAwarded,
	})
	return event
}

func objectiveContextCompletedEvent(
	team *models.Run, objectiveID string, blockContext game.BlockContext,
) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventObjectiveContextCompleted)
	event.ObjectiveID = objectiveID
	event.Data = eventPayload(models.ObjectiveContextCompletedData{Context: blockContext})
	return event
}
