package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

type PhotoBlock struct {
	BaseBlock
	Prompt    string `json:"prompt"`
	MaxImages int    `json:"max_images"`
}

type photoBlockData struct {
	URLs []string `json:"images"`
}

// Basic Attributes Getters

func (b *PhotoBlock) GetID() string         { return b.ID }
func (b *PhotoBlock) GetType() string       { return "photo" }
func (b *PhotoBlock) GetLocationID() string { return b.LocationID }
func (b *PhotoBlock) GetName() string       { return "Photo" }
func (b *PhotoBlock) GetDescription() string {
	return "Players must submit a photo"
}
func (b *PhotoBlock) GetOrder() int  { return b.Order }
func (b *PhotoBlock) GetPoints() int { return b.Points }
func (b *PhotoBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-camera"><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg>`
}
func (b *PhotoBlock) GetAdminData() any {
	return &b
}
func (b *PhotoBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *PhotoBlock) ParseData() error {
	if err := json.Unmarshal(b.Data, b); err != nil {
		return err
	}
	// Set default MaxImages if not set
	if b.MaxImages == 0 {
		b.MaxImages = 1
	}
	return nil
}

func (b *PhotoBlock) UpdateBlockData(input map[string][]string) error {
	// Points
	if input["points"] != nil {
		points, err := strconv.Atoi(input["points"][0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	}
	// Prompt
	if input["prompt"] == nil {
		return errors.New("prompt is a required field")
	}
	b.Prompt = input["prompt"][0]
	// Max Images
	if input["max_images"] != nil {
		maxImages, err := strconv.Atoi(input["max_images"][0])
		if err != nil {
			return errors.New("max_images must be an integer")
		}
		if maxImages < 1 || maxImages > 5 {
			return errors.New("max_images must be between 1 and 5")
		}
		b.MaxImages = maxImages
	} else if b.MaxImages == 0 {
		b.MaxImages = 1 // Default to 1 if not set
	}
	return nil
}

// Validation and Points Calculation

func (b *PhotoBlock) RequiresValidation() bool { return true }

func (b *PhotoBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	newPlayerData := photoBlockData{}
	if state.GetPlayerData() != nil {
		err := json.Unmarshal(state.GetPlayerData(), &newPlayerData)
		if err != nil {
			return state, fmt.Errorf("unmarshalling player data %w", err)
		}
	}

	// Handle delete operation
	if len(input["delete"]) > 0 {
		urlToDelete := input["delete"][0]
		filtered := make([]string, 0, len(newPlayerData.URLs)-1)
		for _, url := range newPlayerData.URLs {
			if url != urlToDelete {
				filtered = append(filtered, url)
			}
		}
		newPlayerData.URLs = filtered

		// Save updated state
		playerData, err := json.Marshal(newPlayerData)
		if err != nil {
			return state, errors.New("error saving player data")
		}
		state.SetPlayerData(playerData)

		// Check if we've reached the max limit for completion
		maxImages := b.MaxImages
		if maxImages == 0 {
			maxImages = 1
		}
		isComplete := len(newPlayerData.URLs) >= maxImages
		state.SetComplete(isComplete)

		if len(newPlayerData.URLs) > 0 && isComplete {
			state.SetPointsAwarded(b.Points)
		} else {
			state.SetPointsAwarded(0)
		}
		return state, nil
	}

	// Handle add operation
	if len(input["url"]) == 0 {
		return state, errors.New("photo is a required field")
	}

	// Check if adding would exceed max limit
	maxImages := b.MaxImages
	if maxImages == 0 {
		maxImages = 1
	}
	if len(newPlayerData.URLs) >= maxImages {
		return state, fmt.Errorf("maximum of %d images allowed", maxImages)
	}

	for _, image := range input["url"] {
		if image == "" {
			return state, errors.New("photo is a required field")
		}
		// Check valid image URL
		if _, err := url.ParseRequestURI(image); err != nil {
			return state, errors.New("invalid URL")
		}
		if len(newPlayerData.URLs) < maxImages {
			newPlayerData.URLs = append(newPlayerData.URLs, image)
		}
	}

	// Trim any excess images beyond the limit
	if len(newPlayerData.URLs) > maxImages {
		newPlayerData.URLs = newPlayerData.URLs[:maxImages]
	}

	// Update state - only mark complete if we've reached the max limit
	playerData, err := json.Marshal(newPlayerData)
	if err != nil {
		return state, errors.New("error saving player data")
	}
	state.SetPlayerData(playerData)

	isComplete := len(newPlayerData.URLs) >= maxImages
	state.SetComplete(isComplete)

	if isComplete {
		state.SetPointsAwarded(b.Points)
	}
	return state, nil
}

// GetImageURLs extracts the image URLs from the player state.
func (b *PhotoBlock) GetImageURLs(state PlayerState) []string {
	if state.GetPlayerData() == nil {
		return []string{}
	}

	var data photoBlockData
	err := json.Unmarshal(state.GetPlayerData(), &data)
	if err != nil {
		return []string{}
	}

	return data.URLs
}
