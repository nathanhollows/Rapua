package models

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

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
