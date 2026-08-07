package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrArray_JSONEncoding(t *testing.T) {
	arr := models.StrArray{"Park Entrance", "Old Tower", "River Bank"}
	jsonVal, err := arr.Value()
	require.NoError(t, err)

	// Expected JSON format
	expected := `["Park Entrance","Old Tower","River Bank"]`
	assert.JSONEq(t, expected, jsonVal.(string))
}

func TestStrArray_JSONDecoding(t *testing.T) {
	var arr models.StrArray

	// Normal JSON case
	err := arr.Scan(`["Park Entrance","Old Tower","River Bank"]`)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{"Park Entrance", "Old Tower", "River Bank"}, arr)

	// Handles empty array
	err = arr.Scan(`[]`)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{}, arr)

	// Handles nil case
	err = arr.Scan(nil)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{}, arr)

	// Handles special characters
	err = arr.Scan(`["This \"quote\" test","Line\nBreak","Tab\tTest"]`)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{`This "quote" test`, "Line\nBreak", "Tab\tTest"}, arr)

	// Invalid JSON case
	err = arr.Scan(`{bad json}`)
	require.Error(t, err)
}

func TestRouteStrategy_String(t *testing.T) {
	tests := []struct {
		strategy models.RouteStrategy
		expected string
	}{
		{models.RouteStrategyRandomised, "Randomised Route"},
		{models.RouteStrategyFreeRoam, "Open Exploration"},
		{models.RouteStrategyOrdered, "Guided Path"},
		{models.RouteStrategySecret, "Secret"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.strategy.String())
	}
}

func TestRouteStrategy_Description(t *testing.T) {
	for _, s := range models.GetRouteStrategies() {
		assert.NotEmpty(t, s.Description(), "missing description for %s", s)
	}
}

func TestParseRouteStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected models.RouteStrategy
		wantErr  bool
	}{
		{"randomised", models.RouteStrategyRandomised, false},
		{"free_roam", models.RouteStrategyFreeRoam, false},
		{"ordered", models.RouteStrategyOrdered, false},
		{"secret", models.RouteStrategySecret, false},
		{"bogus", "", true},
	}
	for _, tt := range tests {
		got, err := models.ParseRouteStrategy(tt.input)
		if tt.wantErr {
			assert.Error(t, err, "input: %s", tt.input)
		} else {
			require.NoError(t, err, "input: %s", tt.input)
			assert.Equal(t, tt.expected, got)
		}
	}
}
