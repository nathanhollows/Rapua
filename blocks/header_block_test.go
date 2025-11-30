package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderBlock_Getters(t *testing.T) {
	block := blocks.HeaderBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Icon:      "https://example.com/logo.png",
		TitleText: "Welcome to the Game",
		TitleSize: "large",
	}

	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
}

func TestHeaderBlock_GetData(t *testing.T) {
	block := blocks.HeaderBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Icon:      "https://example.com/logo.png",
		TitleText: "Welcome",
		TitleSize: "medium",
	}

	data := block.GetData()
	require.NotNil(t, data)

	// Unmarshal to verify structure
	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/logo.png", parsed["icon"])
	assert.Equal(t, "Welcome", parsed["title_text"])
	assert.Equal(t, "medium", parsed["title_size"])
}

func TestHeaderBlock_ParseData(t *testing.T) {
	icon := gofakeit.URL()
	titleText := gofakeit.Sentence(3)
	titleSize := "large"
	data := `{"icon":"` + icon + `","title_text":"` + titleText + `","title_size":"` + titleSize + `"}`

	block := blocks.HeaderBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, icon, block.Icon)
	assert.Equal(t, titleText, block.TitleText)
	assert.Equal(t, titleSize, block.TitleSize)
}

func TestHeaderBlock_ParseData_EmptyJSON(t *testing.T) {
	block := blocks.HeaderBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(`{}`),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Empty(t, block.Icon)
	assert.Empty(t, block.TitleText)
	assert.Empty(t, block.TitleSize)
}

func TestHeaderBlock_ParseData_InvalidJSON(t *testing.T) {
	block := blocks.HeaderBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(`{invalid json}`),
		},
	}

	err := block.ParseData()
	require.Error(t, err)
}

func TestHeaderBlock_UpdateBlockData(t *testing.T) {
	block := blocks.HeaderBlock{}
	data := map[string][]string{
		"icon":       {"https://example.com/new-logo.png"},
		"title_text": {"Updated Title"},
		"title_size": {"small"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/new-logo.png", block.Icon)
	assert.Equal(t, "Updated Title", block.TitleText)
	assert.Equal(t, "small", block.TitleSize)
}

func TestHeaderBlock_UpdateBlockData_OnlyIcon(t *testing.T) {
	block := blocks.HeaderBlock{}
	data := map[string][]string{
		"icon":       {"https://example.com/logo.png"},
		"title_text": {""},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/logo.png", block.Icon)
	assert.Equal(t, "", block.TitleText)
}

func TestHeaderBlock_UpdateBlockData_OnlyTitle(t *testing.T) {
	block := blocks.HeaderBlock{}
	data := map[string][]string{
		"icon":       {""},
		"title_text": {"My Title"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "", block.Icon)
	assert.Equal(t, "My Title", block.TitleText)
}

func TestHeaderBlock_UpdateBlockData_BothEmpty(t *testing.T) {
	block := blocks.HeaderBlock{}
	data := map[string][]string{
		"icon":       {""},
		"title_text": {""},
	}

	err := block.UpdateBlockData(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title text or icon must be provided")
}

func TestHeaderBlock_UpdateBlockData_WithoutTitleSize(t *testing.T) {
	block := blocks.HeaderBlock{
		TitleSize: "large",
	}

	data := map[string][]string{
		"icon":       {"https://example.com/logo.png"},
		"title_text": {"Updated Title"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/logo.png", block.Icon)
	assert.Equal(t, "Updated Title", block.TitleText)
	// title_size should remain unchanged since it wasn't provided
	assert.Equal(t, "large", block.TitleSize)
}

// Note: Test for clearing icon by omitting it is not possible due to implementation
// requiring both icon and title_text to be present in the input map for validation.
// The validation at line 43 of header_block.go accesses icon[0] and titleText[0]
// before checking if they exist in the map, which would cause a panic.

func TestHeaderBlock_RequiresValidation(t *testing.T) {
	block := blocks.HeaderBlock{}
	assert.False(t, block.RequiresValidation())
}

func TestHeaderBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.HeaderBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 5,
		},
		Icon:      "https://example.com/logo.png",
		TitleText: "Game Header",
		TitleSize: "large",
	}

	state := &blocks.MockPlayerState{}

	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)

	// Assert that state is marked as complete
	assert.True(t, newState.IsComplete())
	// Header blocks don't award points
	assert.Equal(t, 0, newState.GetPointsAwarded())
}

func TestHeaderBlock_GetTitleSizes(t *testing.T) {
	block := blocks.HeaderBlock{}
	sizes := block.GetTitleSizes()

	assert.NotNil(t, sizes)
	assert.Len(t, sizes, 3)
	assert.Equal(t, "Small", sizes["small"])
	assert.Equal(t, "Medium", sizes["medium"])
	assert.Equal(t, "Large", sizes["large"])
}
