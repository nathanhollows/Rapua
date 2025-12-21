package models

import (
	"encoding/json"

	"github.com/nathanhollows/Rapua/v6/blocks"
)

type Location struct {
	baseModel

	ID           string  `bun:"id,pk,notnull"`
	Name         string  `bun:"name,type:varchar(255)"`
	InstanceID   string  `bun:"instance_id,notnull"`
	MarkerID     string  `bun:"marker_id,notnull"`
	ContentID    string  `bun:"content_id,notnull"` // TODO: Remove contentID as content is fully deprecated
	Criteria     string  `bun:"criteria,type:varchar(255)"`
	Order        int     `bun:"order,type:int"`
	TotalVisits  int     `bun:"total_visits,type:int"`
	CurrentCount int     `bun:"current_count,type:int"`
	AvgDuration  float64 `bun:"avg_duration,type:float"`
	Points       int     `bun:"points,"`

	Instance Instance `bun:"rel:has-one,join:instance_id=id"`
	Marker   Marker   `bun:"rel:has-one,join:marker_id=code"`
	Blocks   []Block  `bun:"rel:has-many,join:id=owner_id"`
}

// HasCoordinates returns true if the location's marker has coordinates.
func (l *Location) HasCoordinates() bool {
	return l.Marker.IsMapped()
}

// HasCluesContext returns true if the location has any blocks with clues context.
func (l *Location) HasCluesContext() bool {
	for i := range l.Blocks {
		if l.Blocks[i].Context == blocks.ContextLocationClues {
			return true
		}
	}
	return false
}

// HasTaskContext returns true if the location has any blocks with task context and a task name.
func (l *Location) HasTaskContext() bool {
	type task struct {
		TaskName string `json:"task_name"`
	}
	for i := range l.Blocks {
		if l.Blocks[i].Context == blocks.ContextTasks {
			block := l.Blocks[i]
			var t task
			_ = json.Unmarshal(block.Data, &t)
			if t.TaskName != "" {
				return true
			}
		}
	}
	return false
}
