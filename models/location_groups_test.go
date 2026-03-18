package models_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameStructure_JSONRoundTrip(t *testing.T) {
	original := models.GameStructure{
		ID:             "root-id",
		Name:           "",
		Color:          "",
		Routing:        models.RouteStrategyFreeRoam,
		Navigation:     models.NavigationMap,
		CompletionType: models.CompletionAll,
		IsRoot:         true,
		LocationIDs:    []string{"loc-1"},
		SubGroups: []models.GameStructure{
			{
				ID:              "group-1",
				Name:            "Test Group",
				Color:           "primary",
				Routing:         models.RouteStrategyRandomised,
				Navigation:      models.NavigationCustom,
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				MaxNext:         3,
				AutoAdvance:     true,
				LocationIDs:     []string{"loc-2", "loc-3"},
				SubGroups:       []models.GameStructure{},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify string enum values appear in JSON (not integers)
	assert.Contains(t, string(data), `"routing":"free_roam"`)
	assert.Contains(t, string(data), `"navigation":"map"`)
	assert.Contains(t, string(data), `"routing":"randomised"`)
	assert.Contains(t, string(data), `"navigation":"custom"`)

	var decoded models.GameStructure
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Routing, decoded.Routing)
	assert.Equal(t, original.Navigation, decoded.Navigation)
	assert.Equal(t, original.SubGroups[0].Routing, decoded.SubGroups[0].Routing)
	assert.Equal(t, original.SubGroups[0].Navigation, decoded.SubGroups[0].Navigation)
}

func TestGameStructure_ScanValue_RoundTrip(t *testing.T) {
	original := models.GameStructure{
		ID:             "root-id",
		Routing:        models.RouteStrategyOrdered,
		Navigation:     models.NavigationTasks,
		CompletionType: models.CompletionAll,
		LocationIDs:    []string{},
		SubGroups: []models.GameStructure{
			{
				ID:             "secret-group",
				Routing:        models.RouteStrategySecret,
				Navigation:     models.NavigationLabelledMap,
				CompletionType: models.CompletionAll,
				LocationIDs:    []string{},
				SubGroups:      []models.GameStructure{},
			},
		},
	}

	// Value() → string JSON
	val, err := original.Value()
	require.NoError(t, err)
	require.NotNil(t, val)

	jsonStr := val.(string)
	assert.Contains(t, jsonStr, `"ordered"`)
	assert.Contains(t, jsonStr, `"tasks"`)
	assert.Contains(t, jsonStr, `"secret"`)
	assert.Contains(t, jsonStr, `"labelled_map"`)

	// Scan() back
	var scanned models.GameStructure
	err = scanned.Scan(jsonStr)
	require.NoError(t, err)

	assert.Equal(t, models.RouteStrategyOrdered, scanned.Routing)
	assert.Equal(t, models.NavigationTasks, scanned.Navigation)
	assert.Equal(t, models.RouteStrategySecret, scanned.SubGroups[0].Routing)
	assert.Equal(t, models.NavigationLabelledMap, scanned.SubGroups[0].Navigation)
}

func TestGameStructure_AllEnumValues_Serialize(t *testing.T) {
	// Verify every enum constant round-trips through JSON
	for _, rs := range models.GetRouteStrategies() {
		gs := models.GameStructure{
			ID:          "test",
			Routing:     rs,
			LocationIDs: []string{},
			SubGroups:   []models.GameStructure{},
		}
		data, err := json.Marshal(gs)
		require.NoError(t, err, "marshal failed for RouteStrategy %s", rs)

		var decoded models.GameStructure
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err, "unmarshal failed for RouteStrategy %s", rs)
		assert.Equal(t, rs, decoded.Routing)
	}

	for _, nm := range models.GetNavigationModes() {
		gs := models.GameStructure{
			ID:          "test",
			Navigation:  nm,
			LocationIDs: []string{},
			SubGroups:   []models.GameStructure{},
		}
		data, err := json.Marshal(gs)
		require.NoError(t, err, "marshal failed for NavigationMode %s", nm)

		var decoded models.GameStructure
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err, "unmarshal failed for NavigationMode %s", nm)
		assert.Equal(t, nm, decoded.Navigation)
	}
}
