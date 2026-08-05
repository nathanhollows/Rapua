package models

import "time"

// RunVarState stores a single creator-defined variable for a run in a quest.
type RunVarState struct {
	RunCode   string    `bun:"run_code,pk,notnull"`
	QuestID   string    `bun:"quest_id,pk,notnull"`
	VarName   string    `bun:"var_name,pk,notnull"`
	VarValue  string    `bun:"var_value,notnull"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
