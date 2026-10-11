package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
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
// marks the boundary rather than erasing what came before it, and the row carries
// the negative of the total it cleared so the stream still sums to the score.
func runResetEvent(runCode, questID string, clearedPoints int) *models.RunEvent {
	event := eventBase(runCode, questID, models.EventRunReset)
	event.Points = -clearedPoints
	return event
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

// notificationSentEvent records a message being sent to a run, words included:
// the log is the copy that survives a dismissal.
func notificationSentEvent(team *models.Run, notificationID, content string) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventNotificationSent)
	event.Data = eventPayload(models.NotificationData{NotificationID: notificationID, Content: content})
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

// runCompletedEvent records the run finishing. There is no state change of its
// own to ride with, because completion is derived rather than written: the
// paths that can tip the root over record it inside their own transactions,
// where the derivation says the tipping actually happened.
func runCompletedEvent(team *models.Run) *models.RunEvent {
	return eventBase(team.Code, team.QuestID, models.EventRunCompleted)
}

// recordRunCompleted records the finish once per leg. The guard reads inside
// the caller's transaction, where the write lock makes it decisive: a second
// completion racing the first sees the row the first committed and lands as a
// no-op rather than a second ending.
func recordRunCompleted(
	ctx context.Context, tx *bun.Tx, repo repositories.RunEventRepository, team *models.Run,
) error {
	recorded, err := repo.CompletedSinceResetWithTx(ctx, tx, team.Code)
	if err != nil {
		return fmt.Errorf("checking for an earlier finish: %w", err)
	}
	if recorded {
		return nil
	}
	return repo.RecordWithTx(ctx, tx, runCompletedEvent(team))
}

// answeredEvent records one graded answer, right or wrong.
//
// pointsAwarded is what the block computed. Points on the returned event is the
// credited delta, set by the caller.
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

// objectiveContextCompletedEvent records a context clearing, carrying the credit
// that rode the submission which cleared it.
func objectiveContextCompletedEvent(
	team *models.Run, objectiveID string, blockContext game.BlockContext, points int,
) *models.RunEvent {
	event := eventBase(team.Code, team.QuestID, models.EventObjectiveContextCompleted)
	event.ObjectiveID = objectiveID
	event.Data = eventPayload(models.ObjectiveContextCompletedData{Context: blockContext, Points: points})
	return event
}
