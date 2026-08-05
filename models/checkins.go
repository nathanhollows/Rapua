package models

import (
	"time"
)

type CheckIn struct {
	baseModel

	QuestID         string    `bun:"quest_id,notnull"`
	RunID           string    `bun:"run_code,pk,type:string"`
	LocationID      string    `bun:"location_id,pk,type:string"`
	TimeIn          time.Time `bun:"time_in,type:datetime"`
	TimeOut         time.Time `bun:"time_out,type:datetime"`
	MustCheckOut    bool      `bun:"must_check_out"`
	Points          int       `bun:"points,"`
	BlocksCompleted bool      `bun:"blocks_completed,type:int"`

	Location Location `bun:"rel:has-one,join:location_id=id"`
}
