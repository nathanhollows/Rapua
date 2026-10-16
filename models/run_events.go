package models

import (
	"encoding/json"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
)

// EventType names one kind of thing that can happen to a run.
type EventType string

const (
	// EventRunStarted fires when a run starts.
	EventRunStarted       EventType = "run_started"
	EventRunNamed         EventType = "run_named"
	EventObjectivesListed EventType = "objectives_listed"
	// EventObjectiveViewed fires when a player opens an objective they can reach.
	EventObjectiveViewed EventType = "objective_viewed"
	// EventBlockAnswered fires on each graded answer, right or wrong.
	EventBlockAnswered EventType = "block_answered"
	// EventObjectiveContextCompleted fires once per objective context that clears.
	EventObjectiveContextCompleted EventType = "objective_context_completed"
	// EventSectionFinished fires once per section a player chooses to end.
	EventSectionFinished  EventType = "section_finished"
	EventUploadAdded      EventType = "upload_added"
	EventNotificationSent EventType = "notification_sent"
	// EventNotificationRead fires when a player dismisses a message, which is not
	// the same as seeing it.
	EventNotificationRead EventType = "notification_read"
	// EventRunCompleted fires once per run, when its root objective completes.
	// Completion is derived rather than written, so without this row the finish,
	// which is the whole story's last line, has no record anywhere.
	EventRunCompleted EventType = "run_completed"
)

// AllEventTypes is the taxonomy, in the order the events tend to happen. Go
// cannot enumerate constants, so a type added above belongs here too, and a
// test reads this list to insist every reader has decided what to do with each
// one: both surfaces drop what they do not name, and a dropped event is a
// silent hole.
//
//nolint:gochecknoglobals // A definition, not state: one place lists the taxonomy.
var AllEventTypes = []EventType{
	EventRunStarted,
	EventRunNamed,
	EventObjectivesListed,
	EventObjectiveViewed,
	EventBlockAnswered,
	EventObjectiveContextCompleted,
	EventSectionFinished,
	EventUploadAdded,
	EventNotificationSent,
	EventNotificationRead,
	EventRunCompleted,
}

// RunEvent is one thing that happened to one run.
//
// Append-only: nothing updates or deletes a row.
type RunEvent struct {
	ID          int64           `bun:"id,pk,autoincrement"`
	RunCode     string          `bun:"run_code,notnull,type:varchar(36)"`
	QuestID     string          `bun:"quest_id,notnull,type:varchar(36)"`
	Type        EventType       `bun:"type,notnull,type:varchar(48)"`
	ObjectiveID string          `bun:"objective_id,nullzero,type:varchar(36)"`
	BlockID     string          `bun:"block_id,nullzero,type:varchar(36)"`
	CreatedAt   time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	Data        json.RawMessage `bun:"data,type:jsonb"`

	// Points is the change this event made to the run's score: the credited
	// delta, negative for a block that deducts. Summing the column over a run
	// rebuilds that run's total.
	Points int `bun:"points,notnull,default:0"`
}

// BlockAnsweredData is the payload of an EventBlockAnswered row. Completed means
// the submission completed the block, which is not the same as the answer being
// right: only the block knows what right means.
type BlockAnsweredData struct {
	BlockType string `json:"block_type"`
	Completed bool   `json:"completed"`
	// PointsAwarded is what the block computed for this submission. The row's
	// Points column carries what was credited, zero whenever the block did not
	// complete or the gate declined.
	PointsAwarded int `json:"points_awarded"`
	// Submitted is what the player sent when the submission did not solve the
	// block, for blocks that report one (game.SubmissionReporter). A solve
	// carries nothing: the block's state is where the final answer lives.
	Submitted json.RawMessage `json:"submitted,omitempty"`
}

// RunNamedData is the payload of an EventRunNamed row.
type RunNamedData struct {
	Name string `json:"name"`
}

// ObjectiveContextCompletedData is the payload of an
// EventObjectiveContextCompleted row. Proof and reveal complete separately, so an
// objective produces one row each.
//
// Points is the credit that rode the submission which cleared the context, zero
// on paths with no award. It duplicates the answer's ledger row deliberately,
// the same way a sent message records its own words, so a reporting read of the
// completion alone still sees what closed it. The event's own Points column
// stays zero, or summing the stream would count the credit twice.
type ObjectiveContextCompletedData struct {
	Context game.BlockContext `json:"context"`
	Points  int               `json:"points"`
}

// UploadAddedData is the payload of an EventUploadAdded row. The URL may outlive
// the file: deleting a run removes its uploads and leaves the events standing.
type UploadAddedData struct {
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
}

// NotificationData is the payload of both notification events. Content is the
// message text, carried only by notification_sent: the notifications table
// filters dismissed rows out of every read, so the log is the copy of the words
// that survives a dismissal.
type NotificationData struct {
	NotificationID string `json:"notification_id"`
	Content        string `json:"content,omitempty"`
}
