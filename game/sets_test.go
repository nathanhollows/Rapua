package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetsField_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  game.SetsField
	}{
		{
			name:  "map of strings",
			input: `{"clue": "greenhouse"}`,
			want:  game.SetsField{"clue": "greenhouse"},
		},
		{
			name:  "numbers are stringified, not rejected",
			input: `{"score": 40, "ratio": 1.5, "negative": -3}`,
			want:  game.SetsField{"score": "40", "ratio": "1.5", "negative": "-3"},
		},
		{
			name:  "large integers keep full precision",
			input: `{"big": 9007199254740993}`,
			want:  game.SetsField{"big": "9007199254740993"},
		},
		{
			name:  "booleans are stringified",
			input: `{"done": true, "skipped": false}`,
			want:  game.SetsField{"done": "true", "skipped": "false"},
		},
		{
			name:  "mixed types in one object",
			input: `{"clue": "greenhouse", "score": 40, "done": true}`,
			want:  game.SetsField{"clue": "greenhouse", "score": "40", "done": "true"},
		},
		{
			name:  "null value becomes empty",
			input: `{"cleared": null}`,
			want:  game.SetsField{"cleared": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s game.SetsField
			require.NoError(t, json.Unmarshal([]byte(tt.input), &s))
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestSetsField_UnmarshalJSON_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "object values are not scalars", input: `{"nested": {"a": 1}}`},
		{name: "array values are not scalars", input: `{"list": [1, 2]}`},
		{name: "a list is not an object", input: `["found_clue"]`},
		{name: "an empty list is not an object", input: `[]`},
		{name: "a scalar is not an object", input: `"just-a-string"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s game.SetsField
			assert.Error(t, json.Unmarshal([]byte(tt.input), &s))
		})
	}
}

// An unsupported value names the key that carries it, so a large document is
// searchable from the error alone.
func TestSetsField_UnmarshalJSON_ErrorNamesTheOffendingKey(t *testing.T) {
	var s game.SetsField
	err := json.Unmarshal([]byte(`{"nested": {"a": 1}}`), &s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nested")
}

// A JSON null leaves the field untouched, matching json.Unmarshal's own
// behaviour for a null into a map.
func TestSetsField_UnmarshalJSON_NullIsNoOp(t *testing.T) {
	s := game.SetsField{"kept": "value"}
	require.NoError(t, json.Unmarshal([]byte(`null`), &s))
	assert.Equal(t, game.SetsField{"kept": "value"}, s)
}

func TestSetsField_RoundTrips(t *testing.T) {
	var s game.SetsField
	require.NoError(t, json.Unmarshal([]byte(`{"score": 40, "clue": "greenhouse"}`), &s))

	out, err := json.Marshal(s)
	require.NoError(t, err)

	var back game.SetsField
	require.NoError(t, json.Unmarshal(out, &back))
	assert.Equal(t, s, back)
}

// A wrong-shaped "sets" reports what the field must be, so a hand-written
// document is fixable from the error alone.
func TestSetsField_UnmarshalJSON_ErrorNamesTheExpectedShape(t *testing.T) {
	var s game.SetsField
	err := json.Unmarshal([]byte(`["found_clue"]`), &s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an object")
}
