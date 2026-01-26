package models

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID        string    `bun:"id,pk,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`

	GameID  string `bun:"game_id,notnull"`
	TeamID  string `bun:"player_id,notnull"`
	OwnerID string `bun:"owner_id,notnull"`

	Type    string          `bun:"type,notnull"`
	Payload json.RawMessage `bun:"payload,notnull"`
}
