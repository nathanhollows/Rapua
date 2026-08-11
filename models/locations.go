package models

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/nathanhollows/Rapua/v8/game"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type Location struct {
	baseModel

	ID           string           `bun:"id,pk,notnull"`
	Name         string           `bun:"name,type:varchar(255)"`
	Slug         string           `bun:"slug,type:varchar(255)"`
	QuestID      string           `bun:"quest_id,notnull"`
	MarkerID     string           `bun:"marker_id,notnull"`
	Criteria     string           `bun:"criteria,type:varchar(255)"`
	When         *game.WhenClause `bun:"when_clause,type:text,nullzero" json:"when,omitempty"`
	Order        int              `bun:"order,type:int"`
	TotalVisits  int              `bun:"total_visits,type:int"`
	CurrentCount int              `bun:"current_count,type:int"`
	AvgDuration  float64          `bun:"avg_duration,type:float"`
	Points       int              `bun:"points,"`

	Quest  Quest   `bun:"rel:has-one,join:quest_id=id"`
	Marker Marker  `bun:"rel:has-one,join:marker_id=code"`
	Blocks []Block `bun:"rel:has-many,join:id=owner_id"`
}

var slugNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string to a URL-safe slug.
func Slugify(s string) string {
	s = foldCombiningMarks(s)
	s = strings.ToLower(s)
	s = slugNonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// Folds "Ōtākou" to "Otakou" so the sweep above does not take the vowel along
// with the macron. Letters that do not decompose (ø, ł, ß) still fall to it.
// Built per call because a transform.Chain holds state.
func foldCombiningMarks(s string) string {
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		s,
	)
	if err != nil {
		return s
	}
	return folded
}

// HasCoordinates returns true if the location's marker has coordinates.
func (l *Location) HasCoordinates() bool {
	return l.Marker.IsMapped()
}

// HasNavigationContext returns true if the location has any navigation blocks.
func (l *Location) HasNavigationContext() bool {
	for i := range l.Blocks {
		if l.Blocks[i].Context == game.ContextNavigation {
			return true
		}
	}
	return false
}

// HasContentContext returns true if the location has any content blocks.
func (l *Location) HasContentContext() bool {
	for i := range l.Blocks {
		if l.Blocks[i].Context == game.ContextLocationContent {
			return true
		}
	}
	return false
}
