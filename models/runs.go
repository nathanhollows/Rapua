package models

import "time"

type Run struct {
	baseModel

	ID      string `bun:"id,pk"`
	Code    string `bun:"code,unique"`
	Name    string `bun:"name,"`
	QuestID string `bun:"quest_id,notnull"`
	// StartedAt is when the players began the run — zero until then.
	// Distinct from CreatedAt, which is when the run was provisioned.
	StartedAt       time.Time `bun:"started_at,nullzero"`
	HasStarted      bool      `bun:"has_started,default:false"`
	Points          int       `bun:"points,"`
	SkippedGroupIDs []string  `bun:"skipped_group_ids,type:text[],array"`

	Quest    Quest           `bun:"rel:has-one,join:quest_id=id"`
	Messages []Notification  `bun:"rel:has-many,join:code=run_code"`
	Blocks   []RunBlockState `bun:"rel:has-many,join:code=run_code"`

	// VarStates holds creator-defined variable values for this run.
	// Populated by RunService.LoadRelations(); not a DB column.
	VarStates map[string]string `bun:"-"`
}
