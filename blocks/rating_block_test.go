package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRatingBlock_GetName(t *testing.T) {
	block := blocks.RatingBlock{}
	assert.Equal(t, "Rating", block.GetName())
}

func TestRatingBlock_Getters(t *testing.T) {
	block := blocks.RatingBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      2,
			Points:     10,
		},
		Prompt:    "Rate your experience",
		MaxRating: 5,
	}

	assert.Equal(t, "Rating", block.GetName())
	assert.Equal(t, "Players provide a star rating for feedback or assessment.", block.GetDescription())
	assert.Equal(t, "rating", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 2, block.GetOrder())
	assert.Equal(t, 10, block.GetPoints())
	assert.NotEmpty(t, block.GetIconSVG())
}

func TestRatingBlock_ParseData(t *testing.T) {
	data := `{"prompt":"How was the experience?","max_rating":7}`
	block := blocks.RatingBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "How was the experience?", block.Prompt)
	assert.Equal(t, 7, block.MaxRating)
}

func TestRatingBlock_ParseData_DefaultMaxRating(t *testing.T) {
	// When max_rating is 0 (absent or zero in JSON), ParseData should default to 5.
	data := `{"prompt":"Quick rating"}`
	block := blocks.RatingBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, 5, block.MaxRating)
}

func TestRatingBlock_UpdateBlockData(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string][]string
		wantPrompt    string
		wantMaxRating int
		wantPoints    int
		wantErr       bool
		errContains   string
	}{
		{
			name: "all fields populated",
			input: map[string][]string{
				"prompt":     {"Rate your experience"},
				"max_rating": {"5"},
				"points":     {"10"},
			},
			wantPrompt:    "Rate your experience",
			wantMaxRating: 5,
			wantPoints:    10,
		},
		{
			name: "max_rating at minimum boundary",
			input: map[string][]string{
				"prompt":     {"Rate this"},
				"max_rating": {"3"},
			},
			wantPrompt:    "Rate this",
			wantMaxRating: 3,
		},
		{
			name: "max_rating at maximum boundary",
			input: map[string][]string{
				"prompt":     {"Rate this"},
				"max_rating": {"10"},
			},
			wantPrompt:    "Rate this",
			wantMaxRating: 10,
		},
		{
			name: "max_rating absent defaults to 5",
			input: map[string][]string{
				"prompt": {"Quick rating"},
			},
			wantPrompt:    "Quick rating",
			wantMaxRating: 5,
		},
		{
			name: "max_rating empty string defaults to 5",
			input: map[string][]string{
				"prompt":     {"Quick rating"},
				"max_rating": {""},
			},
			wantPrompt:    "Quick rating",
			wantMaxRating: 5,
		},
		{
			name: "zero points allowed",
			input: map[string][]string{
				"prompt": {"Rate this"},
				"points": {"0"},
			},
			wantPrompt:    "Rate this",
			wantMaxRating: 5,
			wantPoints:    0,
		},
		{
			name:        "missing prompt",
			input:       map[string][]string{},
			wantErr:     true,
			errContains: "prompt is required",
		},
		{
			name: "invalid points",
			input: map[string][]string{
				"prompt": {"Rate this"},
				"points": {"not-a-number"},
			},
			wantErr:     true,
			errContains: "points must be an integer",
		},
		{
			name: "max_rating below minimum",
			input: map[string][]string{
				"prompt":     {"Rate this"},
				"max_rating": {"2"},
			},
			wantErr:     true,
			errContains: "max_rating must be between 3 and 10",
		},
		{
			name: "max_rating above maximum",
			input: map[string][]string{
				"prompt":     {"Rate this"},
				"max_rating": {"11"},
			},
			wantErr:     true,
			errContains: "max_rating must be between 3 and 10",
		},
		{
			name: "max_rating not a number",
			input: map[string][]string{
				"prompt":     {"Rate this"},
				"max_rating": {"five"},
			},
			wantErr:     true,
			errContains: "max_rating must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.RatingBlock{}
			err := block.UpdateBlockData(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPrompt, block.Prompt)
			assert.Equal(t, tt.wantMaxRating, block.MaxRating)
			assert.Equal(t, tt.wantPoints, block.Points)
		})
	}
}

func TestRatingBlock_RequiresValidation(t *testing.T) {
	block := blocks.RatingBlock{}
	assert.True(t, block.RequiresValidation())
}

func TestRatingBlock_ValidatePlayerInput(t *testing.T) {
	tests := []struct {
		name          string
		maxRating     int
		points        int
		input         map[string][]string
		wantRating    int
		wantComplete  bool
		wantPoints    int
		wantErr       bool
		errContains   string
	}{
		{
			name:         "valid rating at minimum",
			maxRating:    5,
			points:       10,
			input:        map[string][]string{"rating": {"1"}},
			wantRating:   1,
			wantComplete: true,
			wantPoints:   10,
		},
		{
			name:         "valid rating at maximum",
			maxRating:    5,
			points:       10,
			input:        map[string][]string{"rating": {"5"}},
			wantRating:   5,
			wantComplete: true,
			wantPoints:   10,
		},
		{
			name:         "valid rating mid-range",
			maxRating:    10,
			points:       20,
			input:        map[string][]string{"rating": {"7"}},
			wantRating:   7,
			wantComplete: true,
			wantPoints:   20,
		},
		{
			name:        "missing rating key",
			maxRating:   5,
			input:       map[string][]string{},
			wantErr:     true,
			errContains: "rating is required",
		},
		{
			name:        "non-numeric rating",
			maxRating:   5,
			input:       map[string][]string{"rating": {"five"}},
			wantErr:     true,
			errContains: "rating must be a number",
		},
		{
			name:        "rating below minimum",
			maxRating:   5,
			input:       map[string][]string{"rating": {"0"}},
			wantErr:     true,
			errContains: "rating must be between 1 and 5",
		},
		{
			name:        "rating above max_rating",
			maxRating:   5,
			input:       map[string][]string{"rating": {"6"}},
			wantErr:     true,
			errContains: "rating must be between 1 and 5",
		},
		{
			name:        "negative rating",
			maxRating:   5,
			input:       map[string][]string{"rating": {"-1"}},
			wantErr:     true,
			errContains: "rating must be between 1 and 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.RatingBlock{
				BaseBlock: blocks.BaseBlock{Points: tt.points},
				MaxRating: tt.maxRating,
			}
			state := &blocks.MockPlayerState{}

			newState, err := block.ValidatePlayerInput(state, tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.True(t, newState.IsComplete())
			assert.Equal(t, tt.wantPoints, newState.GetPointsAwarded())

			// Verify the rating was persisted in player data.
			var playerData blocks.RatingBlockData
			require.NoError(t, json.Unmarshal(newState.GetPlayerData(), &playerData))
			assert.Equal(t, tt.wantRating, playerData.Rating)
		})
	}
}

func TestRatingBlock_GetPlayerRating(t *testing.T) {
	block := blocks.RatingBlock{MaxRating: 5}

	t.Run("valid player data", func(t *testing.T) {
		state := &blocks.MockPlayerState{
			PlayerData: json.RawMessage(`{"rating":4}`),
		}
		assert.Equal(t, 4, block.GetPlayerRating(state))
	})

	t.Run("nil player data", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		assert.Equal(t, 0, block.GetPlayerRating(state))
	})

	t.Run("invalid json", func(t *testing.T) {
		state := &blocks.MockPlayerState{
			PlayerData: json.RawMessage(`not-json`),
		}
		assert.Equal(t, 0, block.GetPlayerRating(state))
	})
}
