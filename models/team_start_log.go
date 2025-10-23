package models

import (
	"time"
)

type TeamStartLog struct {
	ID         string    `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UserID     string    `bun:"user_id,notnull,type:varchar(36)"`
	InstanceID string    `bun:"instance_id,notnull,type:varchar(36)"`
	TeamID     string    `bun:"team_id,notnull,type:varchar(36)"`
}
