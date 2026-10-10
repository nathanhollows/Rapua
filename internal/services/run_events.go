package services

import (
	"encoding/json"
	"time"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Building a run event is a pure function of what happened, so these are package
// functions rather than methods on a type: there is no state to hold, and the
// caller already holds the repository that records the result.

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

func eventBase(team *models.Run, eventType models.EventType) *models.RunEvent {
	return &models.RunEvent{
		RunCode:   team.Code,
		QuestID:   team.QuestID,
		Type:      eventType,
		CreatedAt: time.Now().UTC(),
	}
}

// answeredEvent records one graded answer, right or wrong.
//
// pointsAwarded is what the block reported, which the reachability gate may still
// decline to credit.
func answeredEvent(
	team *models.Run, block blocks.Block, objectiveID string, completed bool, pointsAwarded int,
) *models.RunEvent {
	event := eventBase(team, models.EventBlockAnswered)
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
	event := eventBase(team, models.EventObjectiveContextCompleted)
	event.ObjectiveID = objectiveID
	event.Data = eventPayload(models.ObjectiveContextCompletedData{Context: blockContext})
	return event
}
