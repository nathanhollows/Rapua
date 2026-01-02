package models

import (
	"time"

	"github.com/uptrace/bun"
)

// KVScope defines the scope of a key-value store.
type KVScope string

const (
	// KVScopeGame is scoped to the entire game instance, shared by all.
	KVScopeGame KVScope = "game"

	// KVScopeTeam is scoped to a specific team within a game.
	KVScopeTeam KVScope = "team"

	// KVScopePlayer is scoped to a specific player (future use).
	KVScopePlayer KVScope = "player"
)

// GameState stores scoped key-value data as a JSON blob.
// Single table handles all scopes: game, team, player, etc.
type GameState struct {
	bun.BaseModel `bun:"table:game_state"`

	InstanceID string         `bun:"instance_id,pk,type:varchar(36)"`
	Scope      KVScope        `bun:"scope,pk,type:varchar(20)"`
	EntityID   string         `bun:"entity_id,pk,type:varchar(36)"` // empty for game, team_code for team, player_id for player
	Data       map[string]any `bun:"data,type:jsonb,notnull,default:'{}'"`
	Version    int            `bun:"version,type:int,notnull,default:1"`
	UpdatedAt  time.Time      `bun:"updated_at,notnull,default:current_timestamp"`
}
