package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapBlock_Getters(t *testing.T) {
	block := blocks.MapBlock{
		BaseBlock: blocks.BaseBlock{
			ID:      "test-id",
			OwnerID: "location-123",
			Order:   2,
			Points:  0,
		},
		Latitude:  -36.8485,
		Longitude: 174.7633,
		Zoom:      14,
		Caption:   "Auckland City Centre",
	}

	assert.Equal(t, "map", block.GetType())
	assert.Equal(t, "Map", block.GetName())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetOwnerID())
	assert.Equal(t, 2, block.GetOrder())
	assert.Equal(t, 0, block.GetPoints())
	assert.NotEmpty(t, block.GetIconSVG())
	assert.NotEmpty(t, block.GetDescription())
}

func TestMapBlock_ParseData(t *testing.T) {
	data := `{"latitude":-36.8485,"longitude":174.7633,"zoom":14,"caption":"Auckland City Centre"}`
	block := blocks.MapBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.InDelta(t, -36.8485, block.Latitude, 0.0001)
	assert.InDelta(t, 174.7633, block.Longitude, 0.0001)
	assert.InDelta(t, 14.0, block.Zoom, 0.01)
	assert.Equal(t, "Auckland City Centre", block.Caption)
}

func TestMapBlock_UpdateBlockData(t *testing.T) {
	t.Run("updates all fields", func(t *testing.T) {
		block := blocks.MapBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"latitude":  {"-36.8485"},
			"longitude": {"174.7633"},
			"zoom":      {"16"},
			"caption":   {"Near the waterfront"},
		})
		require.NoError(t, err)
		assert.InDelta(t, -36.8485, block.Latitude, 0.0001)
		assert.InDelta(t, 174.7633, block.Longitude, 0.0001)
		assert.InDelta(t, 16.0, block.Zoom, 0.01)
		assert.Equal(t, "Near the waterfront", block.Caption)
	})

	t.Run("invalid latitude", func(t *testing.T) {
		block := blocks.MapBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"latitude": {"not-a-number"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "latitude")
	})

	t.Run("invalid longitude", func(t *testing.T) {
		block := blocks.MapBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"longitude": {"not-a-number"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "longitude")
	})

	t.Run("invalid zoom", func(t *testing.T) {
		block := blocks.MapBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"zoom": {"not-a-number"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "zoom")
	})

	t.Run("empty values are skipped", func(t *testing.T) {
		block := blocks.MapBlock{Latitude: 1.0, Longitude: 2.0}
		err := block.UpdateBlockData(map[string][]string{
			"latitude":  {""},
			"longitude": {""},
		})
		require.NoError(t, err)
		assert.InDelta(t, 1.0, block.Latitude, 0.0001)
		assert.InDelta(t, 2.0, block.Longitude, 0.0001)
	})

	t.Run("show_marker present sets HideMarker false", func(t *testing.T) {
		block := blocks.MapBlock{HideMarker: true}
		err := block.UpdateBlockData(map[string][]string{
			"show_marker": {"on"},
		})
		require.NoError(t, err)
		assert.False(t, block.HideMarker)
	})

	t.Run("show_marker absent sets HideMarker true", func(t *testing.T) {
		block := blocks.MapBlock{}
		err := block.UpdateBlockData(map[string][]string{})
		require.NoError(t, err)
		assert.True(t, block.HideMarker)
	})
}

func TestMapBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.MapBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 0,
		},
		Latitude:  -36.8485,
		Longitude: 174.7633,
		Zoom:      14,
	}

	state := &blocks.MockPlayerState{}
	newState, err := block.ValidatePlayerInput(state, map[string][]string{})
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())
}

func TestMapBlock_RequiresValidation(t *testing.T) {
	block := blocks.MapBlock{}
	assert.False(t, block.RequiresValidation())
}

func TestMapBlock_GetData_RoundTrip(t *testing.T) {
	original := blocks.MapBlock{
		BaseBlock: blocks.BaseBlock{
			ID:      "roundtrip-id",
			OwnerID: "loc-1",
		},
		Latitude:  -41.2865,
		Longitude: 174.7762,
		Zoom:      12,
		Caption:   "Wellington",
	}

	data := original.GetData()
	require.NotNil(t, data)

	restored := blocks.MapBlock{
		BaseBlock: blocks.BaseBlock{Data: data},
	}
	require.NoError(t, restored.ParseData())

	assert.InDelta(t, original.Latitude, restored.Latitude, 0.0001)
	assert.InDelta(t, original.Longitude, restored.Longitude, 0.0001)
	assert.InDelta(t, original.Zoom, restored.Zoom, 0.01)
	assert.Equal(t, original.Caption, restored.Caption)
	assert.Equal(t, original.HideMarker, restored.HideMarker)
}
