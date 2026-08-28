package models_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameStructure_JSONRoundTrip(t *testing.T) {
	original := models.GameStructure{
		ID:             "root-id",
		Name:           "",
		Color:          "",
		Routing:        models.RouteStrategyFreeRoam,
		CompletionType: models.CompletionAll,
		IsRoot:         true,
		LocationIDs:    []string{"loc-1"},
		SubGroups: []models.GameStructure{
			{
				ID:              "group-1",
				Name:            "Test Group",
				Color:           "primary",
				Routing:         models.RouteStrategyRandomised,
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

	assert.Contains(t, string(data), `"routing":"free_roam"`)
	assert.Contains(t, string(data), `"routing":"randomised"`)

	var decoded models.GameStructure
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Routing, decoded.Routing)
	assert.Equal(t, original.SubGroups[0].Routing, decoded.SubGroups[0].Routing)
}

func TestGameStructure_ScanValue_RoundTrip(t *testing.T) {
	original := models.GameStructure{
		ID:             "root-id",
		Routing:        models.RouteStrategyOrdered,
		CompletionType: models.CompletionAll,
		LocationIDs:    []string{},
		SubGroups: []models.GameStructure{
			{
				ID:             "secret-group",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				LocationIDs:    []string{},
				SubGroups:      []models.GameStructure{},
			},
		},
	}

	val, err := original.Value()
	require.NoError(t, err)
	require.NotNil(t, val)

	jsonStr := val.(string)
	assert.Contains(t, jsonStr, `"ordered"`)
	assert.Contains(t, jsonStr, `"secret"`)

	var scanned models.GameStructure
	err = scanned.Scan(jsonStr)
	require.NoError(t, err)

	assert.Equal(t, models.RouteStrategyOrdered, scanned.Routing)
	assert.Equal(t, models.RouteStrategySecret, scanned.SubGroups[0].Routing)
}

func TestGameStructure_RemoveObjectiveID(t *testing.T) {
	t.Run("removes from root and reports true", func(t *testing.T) {
		gs := models.GameStructure{
			ID:           "root",
			IsRoot:       true,
			ObjectiveIDs: []string{"obj-1", "obj-2"},
			SubGroups:    []models.GameStructure{},
		}

		removed := gs.RemoveObjectiveID("obj-1")

		assert.True(t, removed)
		assert.Equal(t, []string{"obj-2"}, gs.ObjectiveIDs)
	})

	t.Run("removes from nested subgroup and reports true", func(t *testing.T) {
		gs := models.GameStructure{
			ID:           "root",
			IsRoot:       true,
			ObjectiveIDs: []string{},
			SubGroups: []models.GameStructure{
				{
					ID:           "group-1",
					ObjectiveIDs: []string{"obj-1", "obj-2"},
					SubGroups:    []models.GameStructure{},
				},
			},
		}

		removed := gs.RemoveObjectiveID("obj-2")

		assert.True(t, removed)
		assert.Equal(t, []string{"obj-1"}, gs.SubGroups[0].ObjectiveIDs)
	})

	t.Run("unknown ID reports false and leaves structure untouched", func(t *testing.T) {
		gs := models.GameStructure{
			ID:           "root",
			IsRoot:       true,
			ObjectiveIDs: []string{"obj-1"},
			SubGroups:    []models.GameStructure{},
		}

		removed := gs.RemoveObjectiveID("does-not-exist")

		assert.False(t, removed)
		assert.Equal(t, []string{"obj-1"}, gs.ObjectiveIDs)
	})
}

func TestGameStructure_AllRouteStrategies_Serialize(t *testing.T) {
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
}
