package models

import (
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
)

// ObjectiveContextCompletion is an append-only completion log: one row means
// one objective context (proof or reveal) has cleared for one run. Completion
// is one-way, so nothing ever updates or deletes a row: presence is the
// record. The primary key makes the INSERT itself the idempotency guard for
// firing that context's sets: if the insert succeeds, this call is the one
// that just completed it; if it conflicts, someone already did. It also
// doubles as the data source for the player's /journal.
type ObjectiveContextCompletion struct {
	RunCode     string            `bun:"run_code,pk,notnull"`
	ObjectiveID string            `bun:"objective_id,pk,notnull"`
	Context     game.BlockContext `bun:"context,pk,notnull,type:varchar(32)"`
	CompletedAt time.Time         `bun:"completed_at,nullzero,notnull,default:current_timestamp"`

	Run       Run       `bun:"rel:has-one,join:run_code=code"`
	Objective Objective `bun:"rel:has-one,join:objective_id=id"`
}
