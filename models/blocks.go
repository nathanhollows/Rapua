package models

import (
	"encoding/json"

	"github.com/nathanhollows/Rapua/v4/blocks"
)

type Block struct {
	ID                 string              `bun:"id,pk,notnull"`
	OwnerID            string              `bun:"owner_id,notnull"`
	Type               string              `bun:"type,type:int"`
	Context            blocks.BlockContext `bun:"context,type:string"`
	Data               json.RawMessage     `bun:"data,type:jsonb"`
	Ordering           int                 `bun:"ordering,type:int"`
	Points             int                 `bun:"points,type:int"`
	ValidationRequired bool                `bun:"validation_required,type:bool"`
}

type TeamBlockState struct {
	baseModel
	TeamCode      string          `bun:"team_code,pk,notnull"`
	BlockID       string          `bun:"block_id,pk,notnull"`
	IsComplete    bool            `bun:"is_complete,type:bool"`
	PointsAwarded int             `bun:"points_awarded,type:int"`
	PlayerData    json.RawMessage `bun:"player_data,type:jsonb"`
}
