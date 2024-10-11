package models

import (
	"encoding/json"
)

type Block struct {
	ID                 string          `bun:"id,pk,notnull"`
	LocationID         string          `bun:"location_id,notnull"`
	Type               string          `bun:"type,type:int"`
	Data               json.RawMessage `bun:"data,type:jsonb"`
	Ordering           int             `bun:"ordering,type:int"`
	Points             int             `bun:"points,type:int"`
	ValidationRequired bool            `bun:"validation_required,type:bool"`
}

type TeamBlockState struct {
	baseModel
	ID            string          `bun:"id,pk,notnull"`
	TeamCode      string          `bun:"team_code,notnull"`
	BlockID       string          `bun:"block_id,notnull"`
	IsComplete    bool            `bun:"is_complete,type:bool"`
	PointsAwarded int             `bun:"points_awarded,type:int"`
	PlayerData    json.RawMessage `bun:"player_data,type:jsonb"`
}