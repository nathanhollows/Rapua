package models

import "time"

// RunVariable records that a run has set a variable. Variables only ever turn
// on, so a row is never updated or deleted: presence is the value. BlockID is
// the block that set it first, kept so the run's history can say what opened a
// gate.
type RunVariable struct {
	RunCode   string    `bun:"run_code,pk,notnull"`
	Name      string    `bun:"name,pk,notnull"`
	BlockID   string    `bun:"block_id,nullzero"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
