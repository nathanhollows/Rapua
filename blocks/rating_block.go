package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

type RatingBlock struct {
	BaseBlock
	Prompt    string `json:"prompt"`
	MaxRating int    `json:"max_rating"` // 5, 7, or 10
}

type ratingBlockData struct {
	Rating      int    `json:"rating"`
	SubmittedAt string `json:"submitted_at"`
}

// Basic Attributes Getters

func (b *RatingBlock) GetID() string         { return b.ID }
func (b *RatingBlock) GetType() string       { return "rating" }
func (b *RatingBlock) GetLocationID() string { return b.LocationID }
func (b *RatingBlock) GetName() string       { return "Rating" }
func (b *RatingBlock) GetDescription() string {
	return "Players provide a star rating for feedback or assessment."
}
func (b *RatingBlock) GetOrder() int  { return b.Order }
func (b *RatingBlock) GetPoints() int { return b.Points }
func (b *RatingBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-star"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>`
}
func (b *RatingBlock) GetAdminData() any {
	return &b
}
func (b *RatingBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *RatingBlock) ParseData() error {
	if b.MaxRating == 0 {
		b.MaxRating = 5 // Default to 5 stars
	}
	return json.Unmarshal(b.Data, b)
}

func (b *RatingBlock) UpdateBlockData(input map[string][]string) error {
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
		return errors.New("prompt is required")
	}
	b.Prompt = input["prompt"][0]

	// Max Rating
	if input["max_rating"] != nil && input["max_rating"][0] != "" {
		maxRating, err := strconv.Atoi(input["max_rating"][0])
		if err != nil {
			return errors.New("max_rating must be an integer")
		}
		if maxRating < 3 || maxRating > 10 {
			return errors.New("max_rating must be between 3 and 10")
		}
		b.MaxRating = maxRating
	} else {
		b.MaxRating = 5 // Default
	}

	return nil
}

// Validation and Points Calculation

func (b *RatingBlock) RequiresValidation() bool { return true }

func (b *RatingBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	if input["rating"] == nil {
		return state, errors.New("rating is required")
	}

	rating, err := strconv.Atoi(input["rating"][0])
	if err != nil {
		return state, errors.New("rating must be a number")
	}

	// Validate rating is within range
	if rating < 1 || rating > b.MaxRating {
		return state, fmt.Errorf("rating must be between 1 and %d", b.MaxRating)
	}

	// Save player data
	newPlayerData := ratingBlockData{
		Rating: rating,
	}

	playerData, err := json.Marshal(newPlayerData)
	if err != nil {
		return state, errors.New("error saving player data")
	}

	state.SetPlayerData(playerData)
	state.SetComplete(true)
	state.SetPointsAwarded(b.Points)
	return state, nil
}

func (b *RatingBlock) GetPlayerRating(state PlayerState) int {
	var data ratingBlockData
	err := json.Unmarshal(state.GetPlayerData(), &data)
	if err != nil {
		return 0
	}
	return data.Rating
}
