package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// A code reaches the app typed, pasted from a message, or in a link, so every way
// of writing it has to find the same run.
func TestNormaliseRunCode(t *testing.T) {
	cases := map[string]string{
		"ABCDEF":    "ABCDEF",
		"abcdef":    "ABCDEF",
		"ABC-DEF":   "ABCDEF",
		" abc def ": "ABCDEF",
		"ABC–DEF":   "ABCDEF",
		"ABCD":      "ABCD",
		"":          "",
	}
	for in, want := range cases {
		assert.Equal(t, want, models.NormaliseRunCode(in), in)
	}
}

// The break is for reading only: a code made before they grew has none.
func TestFormatRunCode(t *testing.T) {
	assert.Equal(t, "ABC-DEF", models.FormatRunCode("ABCDEF"))
	assert.Equal(t, "ABCD", models.FormatRunCode("ABCD"), "a short code is shown as it is")
	assert.Equal(t, "ABCDE", models.FormatRunCode("ABCDE"))
	assert.Equal(t, "ABCDEF", models.NormaliseRunCode(models.FormatRunCode("ABCDEF")), "the break never survives into storage")
}
