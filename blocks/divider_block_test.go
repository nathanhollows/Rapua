package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDividerBlock_Getters(t *testing.T) {
	block := blocks.DividerBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Title: "Test Title",
	}

	assert.Equal(t, "divider", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
}

func TestDividerBlock_ParseData(t *testing.T) {
	data := `{"title":""}`
	block := blocks.DividerBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Empty(t, block.Title)
}

func TestDividerBlock_UpdateBlockData(t *testing.T) {
	title := gofakeit.Word()
	block := blocks.DividerBlock{}
	data := map[string][]string{
		"title": {title},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, title, block.Title)
}

func TestDividerBlock_ValidatePlayerInput(t *testing.T) {
	title := gofakeit.Word()
	block := blocks.DividerBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 5,
		},
		Title: title,
	}

	state := &blocks.MockPlayerState{}

	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)

	// Assert that state is marked as complete
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())
}
