package models

import "github.com/nathanhollows/Rapua/v6/blocks"

type Location struct {
	baseModel

	ID           string  `bun:"id,pk,notnull"`
	Name         string  `bun:"name,type:varchar(255)"`
	InstanceID   string  `bun:"instance_id,notnull"`
	MarkerID     string  `bun:"marker_id,notnull"`
	ContentID    string  `bun:"content_id,notnull"`
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

// HasCheckpointBlocks returns true if the location has any blocks with checkpoint/validation context.
func (l *Location) HasCheckpointBlocks() bool {
	for i := range l.Blocks {
		if l.Blocks[i].Context == blocks.ContextCheckpoint {
			return true
		}
	}
	return false
}
