package models

import (
	"regexp"
	"strings"

	"github.com/nathanhollows/Rapua/v7/game"
)

type Location struct {
	baseModel

	ID           string  `bun:"id,pk,notnull"`
	Name         string  `bun:"name,type:varchar(255)"`
	Slug         string  `bun:"slug,type:varchar(255)"`
	InstanceID   string  `bun:"instance_id,notnull"`
	MarkerID     string  `bun:"marker_id,notnull"`
	Criteria     string          `bun:"criteria,type:varchar(255)"`
	When         *game.WhenClause `bun:"when_clause,type:text,nullzero" json:"when,omitempty"`
	Order        int             `bun:"order,type:int"`
	TotalVisits  int     `bun:"total_visits,type:int"`
	CurrentCount int     `bun:"current_count,type:int"`
	AvgDuration  float64 `bun:"avg_duration,type:float"`
	Points       int     `bun:"points,"`

	Instance Instance `bun:"rel:has-one,join:instance_id=id"`
	Marker   Marker   `bun:"rel:has-one,join:marker_id=code"`
	Blocks   []Block  `bun:"rel:has-many,join:id=owner_id"`
}

var slugNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string to a URL-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = slugNonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// HasCoordinates returns true if the location's marker has coordinates.
func (l *Location) HasCoordinates() bool {
	return l.Marker.IsMapped()
}

// HasCluesContext returns true if the location has any blocks with clues context.
func (l *Location) HasCluesContext() bool {
	for i := range l.Blocks {
		if l.Blocks[i].Context == game.ContextLocationClues {
			return true
		}
	}
	return false
}

// HasTaskContext returns true if the location has any blocks with task context.
func (l *Location) HasTaskContext() bool {
	for i := range l.Blocks {
		if l.Blocks[i].Context == game.ContextTask {
			return true
		}
	}
	return false
}
