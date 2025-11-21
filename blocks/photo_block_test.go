package blocks

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhotoBlock_Getters(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      5,
			Points:     10,
		},
		Prompt: "Take a photo of the landmark",
	}

	assert.Equal(t, "Photo", block.GetName())
	assert.Equal(t, "photo", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 5, block.GetOrder())
	assert.Equal(t, 10, block.GetPoints())
	assert.Contains(t, block.GetDescription(), "submit a photo")
}

func TestPhotoBlock_ValidatePlayerInput_Success(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt: "Take a photo",
	}

	state := &mockPlayerState{
		isComplete: false,
	}

	imageURL := gofakeit.URL()
	input := map[string][]string{
		"url": {imageURL},
	}

	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 10, newState.GetPointsAwarded())

	// Verify the URL was stored in player data
	var data photoBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &data)
	require.NoError(t, err)
	assert.Contains(t, data.URLs, imageURL)
}

func TestPhotoBlock_ValidatePlayerInput_MultiplePhotos(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt: "Take photos",
	}

	state := &mockPlayerState{
		isComplete: false,
	}

	// Submit first photo
	imageURL1 := gofakeit.URL()
	input1 := map[string][]string{
		"url": {imageURL1},
	}

	newState, err := block.ValidatePlayerInput(state, input1)
	require.NoError(t, err)

	// Submit second photo (should append)
	imageURL2 := gofakeit.URL()
	input2 := map[string][]string{
		"url": {imageURL2},
	}

	finalState, err := block.ValidatePlayerInput(newState, input2)
	require.NoError(t, err)

	// Verify both URLs are stored
	var data photoBlockData
	err = json.Unmarshal(finalState.GetPlayerData(), &data)
	require.NoError(t, err)
	assert.Len(t, data.URLs, 2)
	assert.Contains(t, data.URLs, imageURL1)
	assert.Contains(t, data.URLs, imageURL2)
}

func TestPhotoBlock_ValidatePlayerInput_MissingURL(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &mockPlayerState{
		isComplete: false,
	}

	// No URL provided
	input := map[string][]string{}

	_, err := block.ValidatePlayerInput(state, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field")
}

func TestPhotoBlock_ValidatePlayerInput_EmptyURL(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &mockPlayerState{
		isComplete: false,
	}

	// Empty URL
	input := map[string][]string{
		"url": {""},
	}

	_, err := block.ValidatePlayerInput(state, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field")
}

func TestPhotoBlock_ValidatePlayerInput_InvalidURL(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &mockPlayerState{
		isComplete: false,
	}

	// Invalid URL format
	input := map[string][]string{
		"url": {"not-a-valid-url"},
	}

	_, err := block.ValidatePlayerInput(state, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestPhotoBlock_GetImageURLs_Success(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	imageURL1 := gofakeit.URL()
	imageURL2 := gofakeit.URL()

	data := photoBlockData{
		URLs: []string{imageURL1, imageURL2},
	}
	playerData, err := json.Marshal(data)
	require.NoError(t, err)

	state := &mockPlayerState{
		playerData: playerData,
		isComplete: true,
	}

	urls := block.GetImageURLs(state)
	assert.Len(t, urls, 2)
	assert.Contains(t, urls, imageURL1)
	assert.Contains(t, urls, imageURL2)
}

func TestPhotoBlock_GetImageURLs_NoData(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &mockPlayerState{
		playerData: nil,
		isComplete: false,
	}

	urls := block.GetImageURLs(state)
	assert.Empty(t, urls)
}

func TestPhotoBlock_GetImageURLs_InvalidJSON(t *testing.T) {
	block := PhotoBlock{
		BaseBlock: BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &mockPlayerState{
		playerData: []byte("invalid json"),
		isComplete: true,
	}

	urls := block.GetImageURLs(state)
	assert.Empty(t, urls)
}

func TestPhotoBlock_UpdateBlockData(t *testing.T) {
	block := PhotoBlock{}
	prompt := gofakeit.Sentence(10)

	data := map[string][]string{
		"prompt": {prompt},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, prompt, block.Prompt)
}
