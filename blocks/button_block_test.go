package blocks

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestButtonBlock_Getters(t *testing.T) {
	block := ButtonBlock{
		BaseBlock: BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Link:    "https://example.com",
		Text:    "Click me",
		Variant: "primary",
	}

	assert.Equal(t, "button", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
}

func TestButtonBlock_ParseData(t *testing.T) {
	link := gofakeit.URL()
	text := gofakeit.Sentence(2)
	variant := "success"
	data := `{"link":"` + link + `","text":"` + text + `","variant":"` + variant + `"}`

	block := ButtonBlock{
		BaseBlock: BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, link, block.Link)
	assert.Equal(t, text, block.Text)
	assert.Equal(t, variant, block.Variant)
}

func TestButtonBlock_UpdateBlockData(t *testing.T) {
	block := ButtonBlock{}
	data := map[string][]string{
		"link":    {"https://example.com"},
		"text":    {"Updated Button Text"},
		"variant": {"warning"},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", block.Link)
	assert.Equal(t, "Updated Button Text", block.Text)
	assert.Equal(t, "warning", block.Variant)
}

func TestButtonBlock_ValidatePlayerInput(t *testing.T) {
	block := ButtonBlock{
		BaseBlock: BaseBlock{
			Points: 5,
		},
		Link:    "https://example.com",
		Text:    "Test Button",
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
