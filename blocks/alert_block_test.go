package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertBlock_Getters(t *testing.T) {
	block := blocks.AlertBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Content: "Test Content",
	}

	assert.Equal(t, "alert", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
}

func TestAlertBlock_ParseData(t *testing.T) {
	content := gofakeit.Sentence(5)
	variant := gofakeit.Word()
	data := `{"content":"` + content + `","variant":"` + variant + `"}`
	block := blocks.AlertBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, content, block.Content)
	assert.Equal(t, variant, block.Variant)
}

func TestAlertBlock_UpdateBlockData(t *testing.T) {
	block := blocks.AlertBlock{}
	data := map[string][]string{
		"content": {"Updated Content"},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "Updated Content", block.Content)
}

func TestAlertBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.AlertBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 5,
		},
		Content: "Test Content",
		Variant: "info",
	}

	state := &blocks.MockPlayerState{}

	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)

	// Assert that state is marked as complete
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())
}
