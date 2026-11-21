package models

import (
	"strings"
	"time"
)

// NormaliseRunCode is the stored form of a code a person typed, pasted or followed
// a link to: capitals, with no break, space or stray punctuation.
func NormaliseRunCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FormatRunCode is how a code is shown: a long one has a break in the middle so
// it reads in two halves. The break is never stored.
func FormatRunCode(code string) string {
	const (
		breakFrom = 6
		halves    = 2
	)
	if len(code) < breakFrom {
		return code
	}
	half := len(code) / halves
	return code[:half] + "-" + code[half:]
}

type Run struct {
	baseModel

	ID      string `bun:"id,pk"`
	Code    string `bun:"code,unique"`
	Name    string `bun:"name,"`
	QuestID string `bun:"quest_id,notnull"`
	// StartedAt is when the players began the run — zero until then.
	// Distinct from CreatedAt, which is when the run was provisioned.
	StartedAt  time.Time `bun:"started_at,nullzero"`
	HasStarted bool      `bun:"has_started,default:false"`
	Points     int       `bun:"points,"`

	Quest    Quest           `bun:"rel:has-one,join:quest_id=id"`
	Messages []Notification  `bun:"rel:has-many,join:code=run_code"`
	Blocks   []RunBlockState `bun:"rel:has-many,join:code=run_code"`
}
