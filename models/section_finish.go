package models

import "time"

// SectionFinish is an append-only log of players choosing to end a section: one
// row means one run pressed finish on one objective. It follows
// ObjectiveContextCompletion exactly, for the same reason: the press is
// one-way, so nothing updates or deletes a row and presence is the whole
// record. The primary key makes the INSERT its own idempotency guard.
//
// For an objective whose completion band is a range, this row is not a display
// hint but the objective's completion itself: reaching the band's minimum only
// offers the button, and pressing it is what finishes the section.
//
// It is a table of its own rather than another context value on
// ObjectiveContextCompletion because the two record different things. That log
// says a set of blocks has been cleared, and is keyed by which context they
// belong to; this one says a player decided a section was done, which happens
// once per objective and belongs to no context.
type SectionFinish struct {
	RunCode     string    `bun:"run_code,pk,notnull"`
	ObjectiveID string    `bun:"objective_id,pk,notnull"`
	FinishedAt  time.Time `bun:"finished_at,nullzero,notnull,default:current_timestamp"`

	Run       Run       `bun:"rel:has-one,join:run_code=code"`
	Objective Objective `bun:"rel:has-one,join:objective_id=id"`
}
