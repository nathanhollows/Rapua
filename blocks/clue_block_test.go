package blocks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClueBlock_Getters(t *testing.T) {
	block := ClueBlock{
		BaseBlock: BaseBlock{
			ID:         "test-clue-id",
			LocationID: "location-123",
			Order:      3,
			Points:     -15, // Negative points for cost
		},
		ClueText:        "This is the secret clue content",
		DescriptionText: "Need a hint? Click below to reveal.",
		ButtonLabel:     "Get Hint",
	}

	assert.Equal(t, "Clue", block.GetName())
	assert.Equal(t, "clue", block.GetType())
	assert.Equal(t, "test-clue-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 3, block.GetOrder())
	assert.Equal(t, -15, block.GetPoints())
}

func TestClueBlock_ParseData(t *testing.T) {
	data := `{"clue_text":"Secret hint here", "description_text":"Want a clue?", "button_label":"Reveal"}`
	block := ClueBlock{
		BaseBlock: BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Secret hint here", block.ClueText)
	assert.Equal(t, "Want a clue?", block.DescriptionText)
	assert.Equal(t, "Reveal", block.ButtonLabel)
}

func TestClueBlock_UpdateBlockData(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string][]string
		expected ClueBlock
	}{
		{
			name: "update all fields",
			input: map[string][]string{
				"points":           {"-20"},
				"clue_text":        {"The answer is 42"},
				"description_text": {"Having trouble? Get a hint below."},
				"button_label":     {"Show Hint"},
			},
			expected: ClueBlock{
				BaseBlock: BaseBlock{
					Points: -20,
				},
				ClueText:        "The answer is 42",
				DescriptionText: "Having trouble? Get a hint below.",
				ButtonLabel:     "Show Hint",
			},
		},
		{
			name: "default button label when empty",
			input: map[string][]string{
				"points":           {"-10"},
				"clue_text":        {"Hint text"},
				"description_text": {"Description"},
				"button_label":     {""},
			},
			expected: ClueBlock{
				BaseBlock: BaseBlock{
					Points: -10,
				},
				ClueText:        "Hint text",
				DescriptionText: "Description",
				ButtonLabel:     "Reveal Clue",
			},
		},
		{
			name: "zero points when empty",
			input: map[string][]string{
				"points":           {""},
				"clue_text":        {"Some clue"},
				"description_text": {"Some description"},
			},
			expected: ClueBlock{
				BaseBlock: BaseBlock{
					Points: 0,
				},
				ClueText:        "Some clue",
				DescriptionText: "Some description",
				ButtonLabel:     "Reveal Clue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := ClueBlock{}
			err := block.UpdateBlockData(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Points, block.Points)
			assert.Equal(t, tt.expected.ClueText, block.ClueText)
			assert.Equal(t, tt.expected.DescriptionText, block.DescriptionText)
			assert.Equal(t, tt.expected.ButtonLabel, block.ButtonLabel)
		})
	}
}

func TestClueBlock_UpdateBlockData_InvalidPoints(t *testing.T) {
	block := ClueBlock{}
	input := map[string][]string{
		"points": {"invalid"},
	}

	err := block.UpdateBlockData(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "points must be an integer")
}

func TestClueBlock_RequiresValidation(t *testing.T) {
	block := ClueBlock{}
	assert.True(t, block.RequiresValidation())
}

func TestClueBlock_ValidatePlayerInput(t *testing.T) {
	tests := []struct {
		name             string
		block            ClueBlock
		initialState     PlayerState
		input            map[string][]string
		expectedPoints   int
		expectedComplete bool
	}{
		{
			name: "reveal clue with cost",
			block: ClueBlock{
				BaseBlock: BaseBlock{
					Points: -10,
				},
				ClueText: "The secret is here",
			},
			initialState: &mockPlayerState{
				blockID:  "block-1",
				playerID: "player-1",
			},
			input: map[string][]string{
				"reveal_clue": {"true"},
			},
			expectedPoints:   -10,
			expectedComplete: true,
		},
		{
			name: "reveal clue with no cost",
			block: ClueBlock{
				BaseBlock: BaseBlock{
					Points: 0,
				},
				ClueText: "Free clue",
			},
			initialState: &mockPlayerState{
				blockID:  "block-2",
				playerID: "player-2",
			},
			input: map[string][]string{
				"reveal_clue": {"true"},
			},
			expectedPoints:   0,
			expectedComplete: true,
		},
		{
			name: "no reveal input",
			block: ClueBlock{
				BaseBlock: BaseBlock{
					Points: -5,
				},
			},
			initialState: &mockPlayerState{
				blockID:  "block-3",
				playerID: "player-3",
			},
			input:            map[string][]string{},
			expectedPoints:   0,
			expectedComplete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newState, err := tt.block.ValidatePlayerInput(tt.initialState, tt.input)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedComplete, newState.IsComplete())
			assert.Equal(t, tt.expectedPoints, newState.GetPointsAwarded())

			// Check that player data is updated when clue is revealed
			if tt.expectedComplete {
				var playerData clueBlockData
				unmarshalErr := json.Unmarshal(newState.GetPlayerData(), &playerData)
				require.NoError(t, unmarshalErr)
				assert.True(t, playerData.IsRevealed)
			}
		})
	}
}

func TestClueBlock_ValidatePlayerInput_WithExistingPlayerData(t *testing.T) {
	block := ClueBlock{
		BaseBlock: BaseBlock{
			Points: -15,
		},
	}

	// Create state with existing player data
	existingData := clueBlockData{IsRevealed: false}
	existingDataBytes, _ := json.Marshal(existingData)

	initialState := &mockPlayerState{
		blockID:    "block-1",
		playerID:   "player-1",
		playerData: existingDataBytes,
	}

	input := map[string][]string{
		"reveal_clue": {"true"},
	}

	newState, err := block.ValidatePlayerInput(initialState, input)
	require.NoError(t, err)

	assert.True(t, newState.IsComplete())
	assert.Equal(t, -15, newState.GetPointsAwarded())

	// Verify player data is updated
	var updatedData clueBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &updatedData)
	require.NoError(t, err)
	assert.True(t, updatedData.IsRevealed)
}

func TestClueBlock_ValidatePlayerInput_InvalidPlayerData(t *testing.T) {
	block := ClueBlock{}

	// Create state with invalid JSON data
	initialState := &mockPlayerState{
		blockID:    "block-1",
		playerID:   "player-1",
		playerData: json.RawMessage(`invalid json`),
	}

	input := map[string][]string{
		"reveal_clue": {"true"},
	}

	_, err := block.ValidatePlayerInput(initialState, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse player data")
}

func TestClueBlock_GetData(t *testing.T) {
	block := ClueBlock{
		BaseBlock: BaseBlock{
			ID:     "test-id",
			Points: -10,
		},
		ClueText:        "Secret clue",
		DescriptionText: "Description",
		ButtonLabel:     "Custom Label",
	}

	data := block.GetData()
	assert.NotNil(t, data)

	// Verify we can unmarshal the data
	var unmarshaled ClueBlock
	err := json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "Secret clue", unmarshaled.ClueText)
	assert.Equal(t, "Description", unmarshaled.DescriptionText)
	assert.Equal(t, "Custom Label", unmarshaled.ButtonLabel)
}
