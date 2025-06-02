package blocks

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertBlock_Getters(t *testing.T) {
	block := AlertBlock{
		BaseBlock: BaseBlock{
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
	block := AlertBlock{
		BaseBlock: BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, content, block.Content)
	assert.Equal(t, variant, block.Variant)
}

func TestAlertBlock_UpdateBlockData(t *testing.T) {
	block := AlertBlock{}
	data := map[string][]string{
		"content": {"Updated Content"},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "Updated Content", block.Content)
}

func TestAlertBlock_ValidatePlayerInput(t *testing.T) {
	block := AlertBlock{
		BaseBlock: BaseBlock{
			Points: 5,
		},
		Content: "Test Content",
		Variant: "info",
	}

	state := &mockPlayerState{}

	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)

	// Assert that state is marked as complete
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())
}
