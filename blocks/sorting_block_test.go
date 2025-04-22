package blocks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortingBlock_UpdateBlockData(t *testing.T) {
	block := &SortingBlock{
		BaseBlock: BaseBlock{
			ID:   "test-id",
			Type: "sorting",
		},
	}

	// Test with complete input data
	input := map[string][]string{
		"content":          {"Sort these items in chronological order"},
		"points":           {"100"},
		"scoring_scheme":   {AllOrNothing},
		"sorting-items":    {"First item", "Second item", "Third item", "Fourth item"},
		"sorting-item-ids": {"id1", "id2", "id3", "id4"},
	}

	err := block.UpdateBlockData(input)
	assert.NoError(t, err)
	assert.Equal(t, "Sort these items in chronological order", block.Content)
	assert.Equal(t, 100, block.Points)
	assert.Equal(t, AllOrNothing, block.ScoringScheme)
	assert.Len(t, block.Items, 4)
	assert.Equal(t, "id1", block.Items[0].ID)
	assert.Equal(t, "First item", block.Items[0].Description)
	assert.Equal(t, 1, block.Items[0].Position)
}

func TestSortingBlock_ValidatePlayerInput(t *testing.T) {
	t.Run("AllOrNothing scoring", func(t *testing.T) {
		block := createTestSortingBlock(AllOrNothing, 100)

		// Test correct ordering
		state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}
		input := map[string][]string{
			"sorting-item-order": {"id1", "id2", "id3", "id4"},
		}

		newState, err := block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, 100, newState.GetPointsAwarded())

		// Test incorrect ordering - still marks as complete but 0 points
		state = &mockPlayerState{blockID: "block-id", playerID: "player-id"}
		input = map[string][]string{
			"sorting-item-order": {"id1", "id3", "id2", "id4"},
		}

		newState, err = block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)
		assert.True(t, newState.IsComplete())           // Block is marked as complete regardless
		assert.Equal(t, 0, newState.GetPointsAwarded()) // But no points awarded
	})

	t.Run("CorrectItemCorrectPlace scoring", func(t *testing.T) {
		block := createTestSortingBlock(CorrectItemCorrectPlace, 100)

		// Test partially correct ordering
		state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}
		input := map[string][]string{
			"sorting-item-order": {"id1", "id3", "id2", "id4"},
		}

		newState, err := block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, 50, newState.GetPointsAwarded()) // 2 items correct (id1, id4)
	})

	t.Run("RetryUntilCorrect scoring", func(t *testing.T) {
		block := createTestSortingBlock(RetryUntilCorrect, 100)

		// Test incorrect ordering - first attempt
		state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}
		input := map[string][]string{
			"sorting-item-order": {"id1", "id3", "id2", "id4"},
		}

		newState, err := block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)
		assert.False(t, newState.IsComplete())
		assert.Equal(t, 0, newState.GetPointsAwarded())

		// Extract player data to verify attempts were tracked
		var playerData SortingPlayerData
		err = json.Unmarshal(newState.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 1, playerData.Attempts)
		assert.False(t, playerData.IsCorrect)

		// Test correct ordering - second attempt
		input = map[string][]string{
			"sorting-item-order": {"id1", "id2", "id3", "id4"},
		}

		newState, err = block.ValidatePlayerInput(newState, input)
		assert.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, 100, newState.GetPointsAwarded())

		// Extract player data to verify attempts were tracked
		err = json.Unmarshal(newState.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 2, playerData.Attempts)
		assert.True(t, playerData.IsCorrect)

		// Test that further submissions don't change the result
		input = map[string][]string{
			"sorting-item-order": {"id4", "id3", "id2", "id1"}, // Completely wrong order
		}

		finalState, err := block.ValidatePlayerInput(newState, input)
		assert.NoError(t, err)
		assert.True(t, finalState.IsComplete())
		assert.Equal(t, 100, finalState.GetPointsAwarded()) // Points remain the same
	})
}

// Helper function to create a test sorting block
func createTestSortingBlock(scoringScheme string, points int) *SortingBlock {
	return &SortingBlock{
		BaseBlock: BaseBlock{
			ID:     "block-id",
			Type:   "sorting",
			Points: points,
		},
		Content:       "Test sorting items",
		ScoringScheme: scoringScheme,
		Items: []SortingItem{
			{ID: "id1", Description: "Item 1", Position: 1},
			{ID: "id2", Description: "Item 2", Position: 2},
			{ID: "id3", Description: "Item 3", Position: 3},
			{ID: "id4", Description: "Item 4", Position: 4},
		},
	}
}

