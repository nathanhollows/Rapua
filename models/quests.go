package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Quest represents an entire game state.
type Quest struct {
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

	Runs       []Run         `bun:"rel:has-many,join:id=quest_id"`
	Locations  []Location    `bun:"rel:has-many,join:id=quest_id"`
	Objectives []Objective   `bun:"rel:has-many,join:id=quest_id"`
	Settings   QuestSettings `bun:"rel:has-one,join:id=quest_id"`
	ShareLinks []ShareLink   `bun:"rel:has-many,join:id=template_id"`
}

// GetStatus returns the status of the quest.
func (q *Quest) GetStatus() GameStatus {
	// If the start time is null, the game is closed
	if q.StartTime.Time.IsZero() {
		return Closed
	}

	// If the start time is in the future, the game is scheduled
	if q.StartTime.Time.UTC().After(time.Now().UTC()) {
		return Scheduled
	}

	// If the end time is in the past, the game is closed
	if !q.EndTime.Time.IsZero() && q.EndTime.Time.Before(time.Now().UTC()) {
		return Closed
	}

	// If the start time is in the past, the game is active
	return Active
}
