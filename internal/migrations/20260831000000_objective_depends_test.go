package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhenToDepends(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty column", raw: "", want: ""},
		{name: "whitespace only", raw: "   ", want: ""},
		{
			name: "bare truthy check",
			raw:  `{"all_of":[{"var":"found_key"}]}`,
			want: `["found_key"]`,
		},
		{
			name: "several ANDed checks keep their order",
			raw:  `{"all_of":[{"var":"top"},{"var":"heart"},{"var":"base"}]}`,
			want: `["top","heart","base"]`,
		},
		{
			name: "negation becomes the not prefix",
			raw:  `{"all_of":[{"var":"decoy","not":true}]}`,
			want: `["not decoy"]`,
		},
		{
			name: "a comparison has no truthy-only equivalent",
			raw:  `{"all_of":[{"var":"player.points","op":"gte","value":10}]}`,
			want: "",
		},
		{
			name: "one comparison drops the whole clause, not just that condition",
			raw:  `{"all_of":[{"var":"found_key"},{"var":"points","op":"gt","value":3}]}`,
			want: "",
		},
		{
			name: "any_of is an OR, which depends cannot express",
			raw:  `{"all_of":[{"var":"found_key"}],"any_of":[{"var":"shortcut"}]}`,
			want: "",
		},
		{name: "empty clause", raw: `{}`, want: ""},
		{name: "unreadable JSON is dropped, not fatal", raw: `not json`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m20260831000000_whenToDepends(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetsToList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty column is untouched", raw: "", want: ""},
		{
			name: "map becomes a sorted list of its keys",
			raw:  `{"score":"40","clue":"greenhouse"}`,
			want: `["clue","score"]`,
		},
		{
			name: "values are discarded, presence is the whole signal",
			raw:  `{"found":"false"}`,
			want: `["found"]`,
		},
		{
			name: "an already-converted list is left alone",
			raw:  `["clue","score"]`,
			want: `["clue","score"]`,
		},
		{name: "empty map", raw: `{}`, want: `[]`},
		{name: "unreadable JSON is left as-is", raw: `not json`, want: `not json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m20260831000000_setsToList(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
