package blocks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortingBlock_GetterMethods(t *testing.T) {
	block := &SortingBlock{
		BaseBlock: BaseBlock{
			ID:         "test-id",
			Type:       "sorting",
			LocationID: "location-1",
			Order:      5,
			Points:     100,
		},
		Content:       "Test content",
		ScoringScheme: AllOrNothing,
	}

	assert.Equal(t, "Sorting", block.GetName())
	assert.Equal(t, "Sort items in the correct order.", block.GetDescription())
	assert.Contains(t, block.GetIconSVG(), `<svg xmlns="http://www.w3.org/2000/svg"`)
	assert.Equal(t, "sorting", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-1", block.GetLocationID())
	assert.Equal(t, 5, block.GetOrder())
	assert.Equal(t, 100, block.GetPoints())

	// Test GetData method
	data := block.GetData()
	assert.NotNil(t, data)

	var unmarshaled SortingBlock
	err := json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, "Test content", unmarshaled.Content)
	assert.Equal(t, AllOrNothing, unmarshaled.ScoringScheme)
}

func TestSortingBlock_ParseData(t *testing.T) {
	// Create a block with data
	originalBlock := &SortingBlock{
		BaseBlock: BaseBlock{
			ID:   "test-id",
			Type: "sorting",
		},
		Content:       "Test content",
		ScoringScheme: AllOrNothing,
		Items: []SortingItem{
			{ID: "id1", Description: "Item 1", Position: 1},
			{ID: "id2", Description: "Item 2", Position: 2},
		},
	}

	data, err := json.Marshal(originalBlock)
	assert.NoError(t, err)

	// Create a new block and set its Data field
	newBlock := &SortingBlock{
		BaseBlock: BaseBlock{
			ID:   "test-id",
			Type: "sorting",
			Data: data,
		},
	}

	// Parse the data
	err = newBlock.ParseData()
	assert.NoError(t, err)

	// Verify the parsed data
	assert.Equal(t, "Test content", newBlock.Content)
	assert.Equal(t, AllOrNothing, newBlock.ScoringScheme)
	assert.Len(t, newBlock.Items, 2)
	assert.Equal(t, "id1", newBlock.Items[0].ID)
	assert.Equal(t, "Item 1", newBlock.Items[0].Description)
	assert.Equal(t, 1, newBlock.Items[0].Position)
}

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

	// Test with missing points (should default to 0)
	block = &SortingBlock{
		BaseBlock: BaseBlock{
			ID:   "test-id",
			Type: "sorting",
		},
	}
	input = map[string][]string{
		"content":          {"Sort these items in chronological order"},
		"sorting-items":    {"First item", "Second item"},
		"sorting-item-ids": {"id1", "id2"},
	}

	err = block.UpdateBlockData(input)
	assert.NoError(t, err)
	assert.Equal(t, 0, block.Points)

	// Test with empty item descriptions (should be skipped)
	block = &SortingBlock{
		BaseBlock: BaseBlock{
			ID:   "test-id",
			Type: "sorting",
		},
	}
	input = map[string][]string{
		"content":          {"Sort these items"},
		"sorting-items":    {"First item", "", "Third item", ""},
		"sorting-item-ids": {"id1", "id2", "id3", "id4"},
	}

	err = block.UpdateBlockData(input)
	assert.NoError(t, err)
	assert.Len(t, block.Items, 2) // Only non-empty items should be included
	assert.Equal(t, "id1", block.Items[0].ID)
	assert.Equal(t, "id3", block.Items[1].ID)

	// Test with invalid points
	block = &SortingBlock{
		BaseBlock: BaseBlock{
			ID:   "test-id",
			Type: "sorting",
		},
	}
	input = map[string][]string{
		"content":          {"Sort these items"},
		"points":           {"not-a-number"},
		"sorting-items":    {"First item", "Second item"},
		"sorting-item-ids": {"id1", "id2"},
	}

	err = block.UpdateBlockData(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "points must be an integer")
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

		// Test incorrect ordering - second attempt
		input = map[string][]string{
			"sorting-item-order": {"id2", "id1", "id3", "id4"},
		}

		newState2, err := block.ValidatePlayerInput(newState, input)
		assert.NoError(t, err)
		assert.False(t, newState2.IsComplete())
		assert.Equal(t, 0, newState2.GetPointsAwarded())

		// Extract player data to verify attempts were incremented
		err = json.Unmarshal(newState2.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 2, playerData.Attempts)
		assert.False(t, playerData.IsCorrect)

		// Test incorrect ordering - third attempt
		input = map[string][]string{
			"sorting-item-order": {"id4", "id3", "id2", "id1"},
		}

		newState3, err := block.ValidatePlayerInput(newState2, input)
		assert.NoError(t, err)
		assert.False(t, newState3.IsComplete())
		assert.Equal(t, 0, newState3.GetPointsAwarded())

		// Extract player data to verify attempts were incremented
		err = json.Unmarshal(newState3.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 3, playerData.Attempts)
		assert.False(t, playerData.IsCorrect)

		// Test correct ordering - fourth attempt
		input = map[string][]string{
			"sorting-item-order": {"id1", "id2", "id3", "id4"},
		}

		newState4, err := block.ValidatePlayerInput(newState3, input)
		assert.NoError(t, err)
		assert.True(t, newState4.IsComplete())
		assert.Equal(t, 100, newState4.GetPointsAwarded())

		// Extract player data to verify attempts were tracked and marked correct
		err = json.Unmarshal(newState4.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 4, playerData.Attempts)
		assert.True(t, playerData.IsCorrect)

		// Test that further submissions don't change the result
		input = map[string][]string{
			"sorting-item-order": {"id4", "id3", "id2", "id1"}, // Completely wrong order
		}

		finalState, err := block.ValidatePlayerInput(newState4, input)
		assert.NoError(t, err)
		assert.True(t, finalState.IsComplete())
		assert.Equal(t, 100, finalState.GetPointsAwarded()) // Points remain the same

		// Verify data wasn't modified
		err = json.Unmarshal(finalState.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 4, playerData.Attempts) // Attempts should not increase
		assert.True(t, playerData.IsCorrect)    // Should still be marked correct
	})

	t.Run("RetryUntilCorrect with preview mode", func(t *testing.T) {
		block := createTestSortingBlock(RetryUntilCorrect, 100)

		// Create a state and mark IsCorrect=true but not complete
		// This simulates the scenario where the preview middleware might reset completion
		initialPlayerData := SortingPlayerData{
			PlayerOrder:  []string{"id1", "id2", "id3", "id4"},
			ShuffleOrder: []string{"id4", "id3", "id2", "id1"},
			Attempts:     2,
			IsCorrect:    true, // Already correct
		}

		playerDataJSON, err := json.Marshal(initialPlayerData)
		assert.NoError(t, err)

		state := &mockPlayerState{
			blockID:    "block-id",
			playerID:   "player-id",
			playerData: playerDataJSON,
			isComplete: false, // Intentionally not marked complete
		}

		// Submit any order - should be ignored since IsCorrect=true
		input := map[string][]string{
			"sorting-item-order": {"id4", "id3", "id2", "id1"}, // Wrong order
		}

		newState, err := block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)

		// Should remain marked as correct, regardless of the new input
		var playerData SortingPlayerData
		err = json.Unmarshal(newState.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Equal(t, 2, playerData.Attempts) // Attempts should not increase
		assert.True(t, playerData.IsCorrect)    // Still marked as correct
	})

	t.Run("Missing sorting-item-order input", func(t *testing.T) {
		block := createTestSortingBlock(AllOrNothing, 100)
		state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}

		// Test with empty input
		input := map[string][]string{}
		_, err := block.ValidatePlayerInput(state, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sorting order is required")

		// Test with empty array
		input = map[string][]string{
			"sorting-item-order": {},
		}
		_, err = block.ValidatePlayerInput(state, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sorting order is required")
	})

	t.Run("Mismatched item count", func(t *testing.T) {
		block := createTestSortingBlock(AllOrNothing, 100)
		state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}

		// Test with too few items
		input := map[string][]string{
			"sorting-item-order": {"id1", "id2"}, // Only 2 of 4 items
		}
		newState, err := block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, 0, newState.GetPointsAwarded()) // No points awarded for incomplete ordering

		// Test with too many items
		input = map[string][]string{
			"sorting-item-order": {"id1", "id2", "id3", "id4", "id5", "id6"}, // 6 items instead of 4
		}
		newState, err = block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, 0, newState.GetPointsAwarded()) // No points awarded for incorrect ordering
	})

	t.Run("Shuffle order initialization", func(t *testing.T) {
		block := createTestSortingBlock(AllOrNothing, 100)

		// Test that shuffle order is initialized on first attempt
		state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}
		input := map[string][]string{
			"sorting-item-order": {"id1", "id2", "id3", "id4"},
		}

		newState, err := block.ValidatePlayerInput(state, input)
		assert.NoError(t, err)

		var playerData SortingPlayerData
		err = json.Unmarshal(newState.GetPlayerData(), &playerData)
		assert.NoError(t, err)
		assert.Len(t, playerData.ShuffleOrder, 4, "Shuffle order should be initialized with 4 items")

		// Test that same player+block gets the same shuffle order
		state1 := &mockPlayerState{blockID: "block-1", playerID: "player-1"}
		state2 := &mockPlayerState{blockID: "block-1", playerID: "player-1"} // Same player/block

		_, err = block.ValidatePlayerInput(state1, input)
		assert.NoError(t, err)
		_, err = block.ValidatePlayerInput(state2, input)
		assert.NoError(t, err)

		var playerData1, playerData2 SortingPlayerData
		err = json.Unmarshal(state1.GetPlayerData(), &playerData1)
		assert.NoError(t, err)
		err = json.Unmarshal(state2.GetPlayerData(), &playerData2)
		assert.NoError(t, err)

		assert.Equal(t, playerData1.ShuffleOrder, playerData2.ShuffleOrder,
			"Same player+block should get same shuffle order")

		// Test that different players get different shuffle orders
		state3 := &mockPlayerState{blockID: "block-1", playerID: "player-2"} // Different player
		_, err = block.ValidatePlayerInput(state3, input)
		assert.NoError(t, err)

		var playerData3 SortingPlayerData
		err = json.Unmarshal(state3.GetPlayerData(), &playerData3)
		assert.NoError(t, err)

		// This test could potentially fail with very small probability if the shuffle happens to be identical
		// but with 4! = 24 possible orderings, it's unlikely
		assert.NotEqual(t, playerData1.ShuffleOrder, playerData3.ShuffleOrder,
			"Different players should get different shuffle orders")
	})
}

func TestSortingBlock_OrderIsCorrect(t *testing.T) {
	block := createTestSortingBlock(AllOrNothing, 100)

	// Test correct order
	correctOrder := []string{"id1", "id2", "id3", "id4"}
	assert.True(t, block.orderIsCorrect(correctOrder))

	// Test incorrect orders
	incorrectOrder1 := []string{"id2", "id1", "id3", "id4"} // Swapped first two
	assert.False(t, block.orderIsCorrect(incorrectOrder1))

	incorrectOrder2 := []string{"id1", "id2", "id4", "id3"} // Swapped last two
	assert.False(t, block.orderIsCorrect(incorrectOrder2))

	incorrectOrder3 := []string{"id4", "id3", "id2", "id1"} // Completely reversed
	assert.False(t, block.orderIsCorrect(incorrectOrder3))

	// Test with missing items
	tooFewItems := []string{"id1", "id2", "id3"} // Missing last item
	assert.False(t, block.orderIsCorrect(tooFewItems))

	// Test with extra items
	tooManyItems := []string{"id1", "id2", "id3", "id4", "id5"} // Extra item
	assert.False(t, block.orderIsCorrect(tooManyItems))

	// Test with invalid item ID
	invalidItemID := []string{"id1", "id2", "id3", "invalid-id"}
	assert.False(t, block.orderIsCorrect(invalidItemID))
}

func TestSortingBlock_CalculateCorrectItemCorrectPlacePoints(t *testing.T) {
	block := createTestSortingBlock(CorrectItemCorrectPlace, 100)

	// Test all correct - should get full points
	allCorrect := []string{"id1", "id2", "id3", "id4"}
	points := block.calculateCorrectItemCorrectPlacePoints(allCorrect)
	assert.Equal(t, 100, points)

	// Test half correct (first two in correct position)
	halfCorrect := []string{"id1", "id2", "id4", "id3"}
	points = block.calculateCorrectItemCorrectPlacePoints(halfCorrect)
	assert.Equal(t, 50, points)

	// Test one correct
	oneCorrect := []string{"id1", "id3", "id4", "id2"}
	points = block.calculateCorrectItemCorrectPlacePoints(oneCorrect)
	assert.Equal(t, 25, points)

	// Test none correct
	noneCorrect := []string{"id4", "id3", "id2", "id1"}
	points = block.calculateCorrectItemCorrectPlacePoints(noneCorrect)
	assert.Equal(t, 0, points)

	// Test with too few items
	tooFew := []string{"id1", "id2"}
	points = block.calculateCorrectItemCorrectPlacePoints(tooFew)
	assert.Equal(t, 0, points)
}

func TestDeterministicShuffle(t *testing.T) {
	items := []string{"A", "B", "C", "D", "E"}

	// Same seed should produce the same shuffle
	shuffle1 := deterministicShuffle(items, "test-seed-1")
	shuffle2 := deterministicShuffle(items, "test-seed-1")
	assert.Equal(t, shuffle1, shuffle2, "Same seed should produce same shuffle")

	// Test that the shuffle algorithm actually shuffles
	// Just check that the ordering is different from the original
	shuffled := deterministicShuffle(items, "test-seed-1")
	assert.NotEqual(t, items, shuffled, "Shuffle should change the order")

	// Verify the shuffle contains all original items
	assert.ElementsMatch(t, items, shuffle1, "Shuffle should contain all original items")

	// Verify that the original slice isn't modified
	originalCopy := make([]string, len(items))
	copy(originalCopy, items)
	deterministicShuffle(items, "test-seed-1")
	assert.Equal(t, originalCopy, items, "Original slice should not be modified")
}

// Helper function to create a test sorting block.
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
