package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateGroup_ConvertsIntegersToStrings(t *testing.T) {
	input := `{
		"id": "root",
		"routing": 1,
		"navigation": 0,
		"sub_groups": [
			{
				"id": "g1",
				"routing": 0,
				"navigation": 4,
				"sub_groups": []
			},
			{
				"id": "g2",
				"routing": 3,
				"navigation": 5,
				"sub_groups": [
					{
						"id": "g3",
						"routing": 2,
						"navigation": 3,
						"sub_groups": []
					}
				]
			}
		]
	}`

	var group map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &group))

	changed := m20260317_migrateGroup(group)
	assert.True(t, changed)

	// Root
	assert.Equal(t, "free_roam", group["routing"])
	assert.Equal(t, "map", group["navigation"])

	// First subgroup
	subs := group["sub_groups"].([]any)
	g1 := subs[0].(map[string]any)
	assert.Equal(t, "randomised", g1["routing"])
	assert.Equal(t, "custom", g1["navigation"])

	// Second subgroup
	g2 := subs[1].(map[string]any)
	assert.Equal(t, "secret", g2["routing"])
	assert.Equal(t, "tasks", g2["navigation"])

	// Nested subgroup (deprecated clues → custom)
	g3 := g2["sub_groups"].([]any)[0].(map[string]any)
	assert.Equal(t, "ordered", g3["routing"])
	assert.Equal(t, "custom", g3["navigation"])
}

func TestMigrateGroup_AlreadyStrings_NoChange(t *testing.T) {
	input := `{"routing": "ordered", "navigation": "custom", "sub_groups": []}`

	var group map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &group))

	changed := m20260317_migrateGroup(group)
	assert.False(t, changed)
	assert.Equal(t, "ordered", group["routing"])
	assert.Equal(t, "custom", group["navigation"])
}

func TestReverseGroup_ConvertsStringsToIntegers(t *testing.T) {
	input := `{
		"routing": "ordered",
		"navigation": "tasks",
		"sub_groups": [
			{"routing": "randomised", "navigation": "custom", "sub_groups": []}
		]
	}`

	var group map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &group))

	changed := m20260317_reverseGroup(group)
	assert.True(t, changed)

	// JSON numbers decode as float64 — use type-safe comparison via JSON round-trip
	data, err := json.Marshal(group)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"routing":2`)
	assert.Contains(t, string(data), `"navigation":5`)

	// Check subgroup
	subs := group["sub_groups"].([]any)
	subData, err := json.Marshal(subs[0])
	require.NoError(t, err)
	assert.Contains(t, string(subData), `"routing":0`)
	assert.Contains(t, string(subData), `"navigation":4`)
}

func TestMigrateAndReverse_RoundTrip(t *testing.T) {
	input := `{
		"routing": 2,
		"navigation": 1,
		"sub_groups": [
			{"routing": 0, "navigation": 5, "sub_groups": []}
		]
	}`

	var group map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &group))

	assert.True(t, m20260317_migrateGroup(group))
	assert.Equal(t, "ordered", group["routing"])
	assert.Equal(t, "labelled_map", group["navigation"])

	assert.True(t, m20260317_reverseGroup(group))

	data, err := json.Marshal(group)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"routing":2`)
	assert.Contains(t, string(data), `"navigation":1`)
}
