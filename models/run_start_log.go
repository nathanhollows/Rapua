package models

import (
	"time"
)

type RunStartLog struct {
	ID        string    `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UserID    string    `bun:"user_id,notnull,type:varchar(36)"`
	QuestID   string    `bun:"quest_id,notnull,type:varchar(36)"`
	RunID     string    `bun:"run_id,notnull,type:varchar(36)"`
}
