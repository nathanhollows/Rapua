package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stored lists from before variables were removed must parse as the single
// name a block now carries, or every page holding one fails to load.
func TestBlockSetsName_ListsBecomeOneName(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261018000000")
	ctx := context.Background()

	cases := []struct {
		blockType string
		data      string
		want      any // nil means the key is gone
	}{
		{"quiz", `{"question":"Q","sets":["met_guide","spare"]}`, "met_guide"},
		{"free_text", `{"sets":"already_one"}`, "already_one"},
		{"quiz", `{"sets":[]}`, nil},
		{"text", `{"content":"Hi","sets":["ignored"]}`, nil},
		{"team_name", `{"sets":["never_fires"]}`, nil},
	}
	ids := make([]string, len(cases))
	for i, c := range cases {
		ids[i] = gofakeit.UUID()
		seedBlock(t, dbc, ids[i], c.blockType, c.data)
	}

	require.NoError(t, m20261020000000_up(ctx, dbc))
	require.NoError(t, m20261020000000_up(ctx, dbc), "running it again changes nothing")

	for i, c := range cases {
		_, data, found := blockRow(t, dbc, ids[i])
		require.True(t, found)
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(data), &fields))
		got, has := fields["sets"]
		if c.want == nil {
			assert.False(t, has, "%s %s", c.blockType, c.data)
			continue
		}
		assert.Equal(t, c.want, got, "%s %s", c.blockType, c.data)
	}
}
