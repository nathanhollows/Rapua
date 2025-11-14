package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Instance represents an entire game state.
type Instance struct {
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID                    string        `bun:"id,pk,type:varchar(36)"`
	Name                  string        `bun:"name,type:varchar(255)"`
	UserID                string        `bun:"user_id,type:varchar(36)"`
	IsTemplate            bool          `bun:"is_template,type:bool"`
	TemplateID            string        `bun:"template_id,type:varchar(36),nullzero"`
	StartTime             bun.NullTime  `bun:"start_time,nullzero"`
	EndTime               bun.NullTime  `bun:"end_time,nullzero"`
	Status                GameStatus    `bun:"-"`
	IsQuickStartDismissed bool          `bun:"is_quick_start_dismissed,type:bool"`
	GameStructure         GameStructure `bun:"game_structure,type:string"`

	Teams      []Team           `bun:"rel:has-many,join:id=instance_id"`
	Locations  []Location       `bun:"rel:has-many,join:id=instance_id"`
	Settings   InstanceSettings `bun:"rel:has-one,join:id=instance_id"`
	ShareLinks []ShareLink      `bun:"rel:has-many,join:id=template_id"`
}

// GetStatus returns the status of the instance.
func (i *Instance) GetStatus() GameStatus {
	// If the start time is null, the game is closed
	if i.StartTime.Time.IsZero() {
		return Closed
	}

	// If the start time is in the future, the game is scheduled
	if i.StartTime.Time.UTC().After(time.Now().UTC()) {
		return Scheduled
	}

	// If the end time is in the past, the game is closed
	if !i.EndTime.Time.IsZero() && i.EndTime.Time.Before(time.Now().UTC()) {
		return Closed
	}

	// If the start time is in the past, the game is active
	return Active
}
