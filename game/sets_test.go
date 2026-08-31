package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetsField_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  game.SetsField
	}{
		{name: "list of names", input: `["clue", "score"]`, want: game.SetsField{"clue", "score"}},
		{name: "single name", input: `["clue"]`, want: game.SetsField{"clue"}},
		{name: "empty list", input: `[]`, want: game.SetsField{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s game.SetsField
			require.NoError(t, json.Unmarshal([]byte(tt.input), &s))
			assert.Equal(t, tt.want, s)
		})
	}
}

// A map of name to value is rejected outright rather than silently coerced to
// a list of names, so a stale document fails loudly at import.
func TestSetsField_UnmarshalJSON_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "an object is not a list", input: `{"clue": "greenhouse"}`},
		{name: "an empty object is not a list", input: `{}`},
		{name: "a scalar is not a list", input: `"just-a-string"`},
		{name: "numbers are not names", input: `[1, 2]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s game.SetsField
			assert.Error(t, json.Unmarshal([]byte(tt.input), &s))
		})
	}
}

func TestSetsField_RoundTrips(t *testing.T) {
	var s game.SetsField
	require.NoError(t, json.Unmarshal([]byte(`["score", "clue"]`), &s))

	out, err := json.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, `["score","clue"]`, string(out))
}
