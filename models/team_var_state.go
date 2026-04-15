package models

import "time"

// TeamVarState stores a single creator-defined variable for a team in a game instance.
type TeamVarState struct {
	TeamCode   string    `bun:"team_code,pk,notnull"`
	InstanceID string    `bun:"instance_id,pk,notnull"`
	VarName    string    `bun:"var_name,pk,notnull"`
	VarValue   string    `bun:"var_value,notnull"`
	UpdatedAt  time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
