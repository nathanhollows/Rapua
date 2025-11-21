package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhotoBlock_Getters(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
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
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt:    "Take a photo",
		MaxImages: 1,
	}

	state := &blocks.MockPlayerState{
		IsCompleteVal: false,
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
	var data blocks.PhotoBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &data)
	require.NoError(t, err)
	assert.Contains(t, data.URLs, imageURL)
}

func TestPhotoBlock_ValidatePlayerInput_MultiplePhotos(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt:    "Take photos",
		MaxImages: 3,
	}

	state := &blocks.MockPlayerState{
		IsCompleteVal: false,
	}

	// Submit first photo - should not complete yet
	imageURL1 := gofakeit.URL()
	input1 := map[string][]string{
		"url": {imageURL1},
	}

	newState, err := block.ValidatePlayerInput(state, input1)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Submit second photo - should not complete yet
	imageURL2 := gofakeit.URL()
	input2 := map[string][]string{
		"url": {imageURL2},
	}

	secondState, err := block.ValidatePlayerInput(newState, input2)
	require.NoError(t, err)
	assert.False(t, secondState.IsComplete())
	assert.Equal(t, 0, secondState.GetPointsAwarded())

	// Submit third photo - should complete and award points
	imageURL3 := gofakeit.URL()
	input3 := map[string][]string{
		"url": {imageURL3},
	}

	finalState, err := block.ValidatePlayerInput(secondState, input3)
	require.NoError(t, err)
	assert.True(t, finalState.IsComplete())
	assert.Equal(t, 10, finalState.GetPointsAwarded())

	// Verify all URLs are stored
	var data blocks.PhotoBlockData
	err = json.Unmarshal(finalState.GetPlayerData(), &data)
	require.NoError(t, err)
	assert.Len(t, data.URLs, 3)
	assert.Contains(t, data.URLs, imageURL1)
	assert.Contains(t, data.URLs, imageURL2)
	assert.Contains(t, data.URLs, imageURL3)
}

func TestPhotoBlock_ValidatePlayerInput_MissingURL(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &blocks.MockPlayerState{
		IsCompleteVal: false,
	}

	// No URL provided
	input := map[string][]string{}

	_, err := block.ValidatePlayerInput(state, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field")
}

func TestPhotoBlock_ValidatePlayerInput_EmptyURL(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &blocks.MockPlayerState{
		IsCompleteVal: false,
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
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &blocks.MockPlayerState{
		IsCompleteVal: false,
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
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	imageURL1 := gofakeit.URL()
	imageURL2 := gofakeit.URL()

	data := blocks.PhotoBlockData{
		URLs: []string{imageURL1, imageURL2},
	}
	playerData, err := json.Marshal(data)
	require.NoError(t, err)

	state := &blocks.MockPlayerState{
		PlayerData: playerData,
		IsCompleteVal: true,
	}

	urls := block.GetImageURLs(state)
	assert.Len(t, urls, 2)
	assert.Contains(t, urls, imageURL1)
	assert.Contains(t, urls, imageURL2)
}

func TestPhotoBlock_GetImageURLs_NoData(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &blocks.MockPlayerState{
		PlayerData: nil,
		IsCompleteVal: false,
	}

	urls := block.GetImageURLs(state)
	assert.Empty(t, urls)
}

func TestPhotoBlock_GetImageURLs_InvalidJSON(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Prompt:    "Take a photo",
	}

	state := &blocks.MockPlayerState{
		PlayerData: []byte("invalid json"),
		IsCompleteVal: true,
	}

	urls := block.GetImageURLs(state)
	assert.Empty(t, urls)
}

func TestPhotoBlock_UpdateBlockData(t *testing.T) {
	block := blocks.PhotoBlock{}
	prompt := gofakeit.Sentence(10)

	data := map[string][]string{
		"prompt":     {prompt},
		"max_images": {"3"},
		"points":     {"15"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, prompt, block.Prompt)
	assert.Equal(t, 3, block.MaxImages)
	assert.Equal(t, 15, block.Points)
}

func TestPhotoBlock_UpdateBlockData_DefaultMaxImages(t *testing.T) {
	block := blocks.PhotoBlock{}

	data := map[string][]string{
		"prompt": {"Test prompt"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, 1, block.MaxImages) // Should default to 1
}

func TestPhotoBlock_UpdateBlockData_InvalidMaxImages(t *testing.T) {
	tests := []struct {
		name     string
		maxImages string
		wantErr  bool
	}{
		{"too low", "0", true},
		{"too high", "6", true},
		{"negative", "-1", true},
		{"valid min", "1", false},
		{"valid max", "5", false},
		{"not a number", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.PhotoBlock{}
			data := map[string][]string{
				"prompt":     {"Test"},
				"max_images": {tt.maxImages},
			}

			err := block.UpdateBlockData(data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPhotoBlock_ValidatePlayerInput_MaxImagesLimit(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt:    "Take photos",
		MaxImages: 2,
	}

	state := &blocks.MockPlayerState{
		IsCompleteVal: false,
	}

	// Submit first photo
	imageURL1 := gofakeit.URL()
	input1 := map[string][]string{"url": {imageURL1}}
	newState, err := block.ValidatePlayerInput(state, input1)
	require.NoError(t, err)

	// Submit second photo - should complete
	imageURL2 := gofakeit.URL()
	input2 := map[string][]string{"url": {imageURL2}}
	finalState, err := block.ValidatePlayerInput(newState, input2)
	require.NoError(t, err)
	assert.True(t, finalState.IsComplete())

	// Try to submit third photo - should be rejected
	imageURL3 := gofakeit.URL()
	input3 := map[string][]string{"url": {imageURL3}}
	_, err = block.ValidatePlayerInput(finalState, input3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}

func TestPhotoBlock_ValidatePlayerInput_Delete(t *testing.T) {
	imageURL1 := gofakeit.URL()
	imageURL2 := gofakeit.URL()

	existingData := blocks.PhotoBlockData{
		URLs: []string{imageURL1, imageURL2},
	}
	playerData, _ := json.Marshal(existingData)

	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt:    "Take photos",
		MaxImages: 3,
	}

	state := &blocks.MockPlayerState{
		PlayerData: playerData,
		IsCompleteVal: false,
	}

	// Delete first image
	input := map[string][]string{
		"delete": {imageURL1},
	}

	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete()) // Not complete, still below max

	// Verify URL was removed
	var data blocks.PhotoBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &data)
	require.NoError(t, err)
	assert.Len(t, data.URLs, 1)
	assert.NotContains(t, data.URLs, imageURL1)
	assert.Contains(t, data.URLs, imageURL2)
}

func TestPhotoBlock_ValidatePlayerInput_DeleteLastImage(t *testing.T) {
	imageURL := gofakeit.URL()
	existingData := blocks.PhotoBlockData{
		URLs: []string{imageURL},
	}
	playerData, _ := json.Marshal(existingData)

	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-block",
			Points: 10,
		},
		Prompt:    "Take a photo",
		MaxImages: 1,
	}

	state := &blocks.MockPlayerState{
		PlayerData:    playerData,
		IsCompleteVal:    true,
		PointsAwarded: 10,
	}

	// Delete the only image
	input := map[string][]string{
		"delete": {imageURL},
	}

	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded()) // Points should be removed

	// Verify URL was removed
	var data blocks.PhotoBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &data)
	require.NoError(t, err)
	assert.Empty(t, data.URLs)
}

func TestPhotoBlock_ParseData_DefaultMaxImages(t *testing.T) {
	block := blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block",
		},
		Prompt: "Take a photo",
		// MaxImages not set
	}

	data, _ := json.Marshal(block)
	block.Data = data

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, 1, block.MaxImages) // Should default to 1
}
