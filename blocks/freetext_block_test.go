package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFreeTextBlock_Getters(t *testing.T) {
	block := blocks.FreeTextBlock{
		BaseBlock: blocks.BaseBlock{
			ID:      "test-id",
			OwnerID: "location-123",
			Order:   3,
			Points:  5,
		},
		Prompt:      "Describe what you smell",
		Placeholder: "Your thoughts here...",
	}

	assert.Equal(t, "Free Text", block.GetName())
	assert.Equal(t, "Players write a free text response", block.GetDescription())
	assert.Equal(t, "free_text", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetOwnerID())
	assert.Equal(t, 3, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
	assert.NotEmpty(t, block.GetIconSVG())
}

func TestFreeTextBlock_ParseData(t *testing.T) {
	data := `{"prompt":"What do you see?","placeholder":"Look around..."}`
	block := blocks.FreeTextBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "What do you see?", block.Prompt)
	assert.Equal(t, "Look around...", block.Placeholder)
}

func TestFreeTextBlock_UpdateBlockData(t *testing.T) {
	tests := []struct {
		name            string
		input           map[string][]string
		wantPrompt      string
		wantPlaceholder string
		wantPoints      int
		wantErr         bool
		errContains     string
	}{
		{
			name: "all fields populated",
			input: map[string][]string{
				"prompt":      {"Describe the scent"},
				"placeholder": {"e.g. floral, citrus..."},
				"points":      {"10"},
			},
			wantPrompt:      "Describe the scent",
			wantPlaceholder: "e.g. floral, citrus...",
			wantPoints:      10,
		},
		{
			name: "prompt only",
			input: map[string][]string{
				"prompt": {"What do you think?"},
			},
			wantPrompt: "What do you think?",
		},
		{
			name: "zero points allowed",
			input: map[string][]string{
				"prompt": {"Write something"},
				"points": {"0"},
			},
			wantPrompt: "Write something",
			wantPoints: 0,
		},
		{
			name: "invalid points",
			input: map[string][]string{
				"points": {"not-a-number"},
			},
			wantErr:     true,
			errContains: "points must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.FreeTextBlock{}
			err := block.UpdateBlockData(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPrompt, block.Prompt)
			assert.Equal(t, tt.wantPlaceholder, block.Placeholder)
			assert.Equal(t, tt.wantPoints, block.Points)
		})
	}
}

func TestFreeTextBlock_RequiresValidation(t *testing.T) {
	block := blocks.FreeTextBlock{}
	assert.True(t, block.RequiresValidation())
}

func TestFreeTextBlock_ValidatePlayerInput(t *testing.T) {
	tests := []struct {
		name         string
		points       int
		input        map[string][]string
		wantResponse string
		wantComplete bool
		wantPoints   int
	}{
		{
			name:         "valid response with points",
			points:       10,
			input:        map[string][]string{"response": {"I smell citrus and herbs"}},
			wantResponse: "I smell citrus and herbs",
			wantComplete: true,
			wantPoints:   10,
		},
		{
			name:         "valid response zero points",
			points:       0,
			input:        map[string][]string{"response": {"Some thoughts"}},
			wantResponse: "Some thoughts",
			wantComplete: true,
			wantPoints:   0,
		},
		{
			name:         "empty response not complete",
			points:       10,
			input:        map[string][]string{"response": {""}},
			wantResponse: "",
			wantComplete: false,
			wantPoints:   0,
		},
		{
			name:         "missing response key not complete",
			points:       10,
			input:        map[string][]string{},
			wantResponse: "",
			wantComplete: false,
			wantPoints:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.FreeTextBlock{
				BaseBlock: blocks.BaseBlock{Points: tt.points},
			}
			state := &blocks.MockPlayerState{}

			newState, err := block.ValidatePlayerInput(state, tt.input)
			require.NoError(t, err)

			assert.Equal(t, tt.wantComplete, newState.IsComplete())
			assert.Equal(t, tt.wantPoints, newState.GetPointsAwarded())

			// Verify the response was persisted in player data.
			var playerData blocks.FreeTextPlayerData
			require.NoError(t, json.Unmarshal(newState.GetPlayerData(), &playerData))
			assert.Equal(t, tt.wantResponse, playerData.Response)
		})
	}
}

func TestFreeTextBlock_GetResponse(t *testing.T) {
	block := blocks.FreeTextBlock{}

	t.Run("valid player data", func(t *testing.T) {
		state := &blocks.MockPlayerState{
			PlayerData: json.RawMessage(`{"response":"Fresh and lemony"}`),
		}
		assert.Equal(t, "Fresh and lemony", block.GetResponse(state))
	})

	t.Run("nil state", func(t *testing.T) {
		assert.Empty(t, block.GetResponse(nil))
	})

	t.Run("nil player data", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		assert.Empty(t, block.GetResponse(state))
	})

	t.Run("invalid json", func(t *testing.T) {
		state := &blocks.MockPlayerState{
			PlayerData: json.RawMessage(`not-json`),
		}
		assert.Empty(t, block.GetResponse(state))
	})
}

func TestFreeTextBlock_ToYAML(t *testing.T) {
	t.Run("with placeholder", func(t *testing.T) {
		block := blocks.FreeTextBlock{
			Prompt:      "Describe it",
			Placeholder: "Your answer...",
		}
		m := block.ToYAML()
		assert.Equal(t, "Describe it", m["prompt"])
		assert.Equal(t, "Your answer...", m["placeholder"])
	})

	t.Run("without placeholder", func(t *testing.T) {
		block := blocks.FreeTextBlock{
			Prompt: "Describe it",
		}
		m := block.ToYAML()
		assert.Equal(t, "Describe it", m["prompt"])
		_, exists := m["placeholder"]
		assert.False(t, exists)
	})
}
