package blocks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuizBlock_Getters(t *testing.T) {
	block := QuizBlock{
		BaseBlock: BaseBlock{
			ID:         "test-quiz-id",
			LocationID: "location-123",
			Order:      3,
			Points:     50,
		},
		Question:       "What is the capital of France?",
		MultipleChoice: false,
		RandomizeOrder: true,
		RetryEnabled:   false,
	}

	assert.Equal(t, "quiz_block", block.GetType())
	assert.Equal(t, "test-quiz-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 3, block.GetOrder())
	assert.Equal(t, 50, block.GetPoints())
	assert.Contains(t, block.GetIconSVG(), "svg")
}

func TestQuizBlock_ParseData(t *testing.T) {
	data := `{
		"question": "What is 2+2?",
		"options": [
			{"id": "option_0", "text": "3", "is_correct": false, "order": 0},
			{"id": "option_1", "text": "4", "is_correct": true, "order": 1}
		],
		"multiple_choice": false,
		"randomize_order": true,
		"retry_enabled": false
	}`

	block := QuizBlock{
		BaseBlock: BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "What is 2+2?", block.Question)
	assert.Len(t, block.Options, 2)
	assert.Equal(t, "3", block.Options[0].Text)
	assert.False(t, block.Options[0].IsCorrect)
	assert.Equal(t, "4", block.Options[1].Text)
	assert.True(t, block.Options[1].IsCorrect)
	assert.False(t, block.MultipleChoice)
	assert.True(t, block.RandomizeOrder)
	assert.False(t, block.RetryEnabled)
}

func TestQuizBlock_UpdateBlockData(t *testing.T) {
	// Test with single choice quiz
	block := QuizBlock{}
	data := map[string][]string{
		"question":        {"What is the capital of France?"},
		"points":          {"100"},
		"multiple_choice": {},
		"randomize_order": {"on"},
		"retry_enabled":   {},
		"option_text":     {"Paris", "London", "Berlin", "Madrid"},
		"option_correct":  {"option_0", "option_3"}, // Mark Paris and Madrid as correct
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "What is the capital of France?", block.Question)
	assert.Equal(t, 100, block.Points)
	assert.False(t, block.MultipleChoice)
	assert.True(t, block.RandomizeOrder)
	assert.False(t, block.RetryEnabled)
	assert.Len(t, block.Options, 4)
	assert.Equal(t, "Paris", block.Options[0].Text)
	assert.True(t, block.Options[0].IsCorrect)
	assert.Equal(t, "London", block.Options[1].Text)
	assert.False(t, block.Options[1].IsCorrect)
	assert.Equal(t, "Madrid", block.Options[3].Text)
	assert.True(t, block.Options[3].IsCorrect)

	// Test with multiple choice enabled
	block = QuizBlock{}
	data = map[string][]string{
		"question":        {"Select all programming languages:"},
		"points":          {"50"},
		"multiple_choice": {"on"},
		"randomize_order": {},
		"retry_enabled":   {"on"},
		"option_text":     {"Python", "HTML", "JavaScript", "CSS"},
		"option_correct":  {"option_0", "option_2"}, // Python and JavaScript
	}

	err = block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "Select all programming languages:", block.Question)
	assert.Equal(t, 50, block.Points)
	assert.True(t, block.MultipleChoice)
	assert.False(t, block.RandomizeOrder)
	assert.True(t, block.RetryEnabled)
	assert.Len(t, block.Options, 4)
	assert.True(t, block.Options[0].IsCorrect)  // Python
	assert.False(t, block.Options[1].IsCorrect) // HTML
	assert.True(t, block.Options[2].IsCorrect)  // JavaScript
	assert.False(t, block.Options[3].IsCorrect) // CSS

	// Test with empty options (should be filtered out)
	block = QuizBlock{}
	data = map[string][]string{
		"question":       {"Test question"},
		"option_text":    {"Valid option", "", "Another valid", ""},
		"option_correct": {"option_0"},
	}

	err = block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Len(t, block.Options, 2)
	assert.Equal(t, "Valid option", block.Options[0].Text)
	assert.Equal(t, "Another valid", block.Options[1].Text)
}

func TestQuizBlock_RequiresValidation(t *testing.T) {
	block := QuizBlock{}
	assert.True(t, block.RequiresValidation())
}

func TestQuizBlock_ValidatePlayerInput_SingleChoice(t *testing.T) {
	block := QuizBlock{
		BaseBlock: BaseBlock{
			Points: 100,
		},
		Question:       "What is 2+2?",
		MultipleChoice: false,
		RetryEnabled:   false,
		Options: []QuizOption{
			{ID: "option_0", Text: "3", IsCorrect: false},
			{ID: "option_1", Text: "4", IsCorrect: true},
			{ID: "option_2", Text: "5", IsCorrect: false},
		},
	}

	state := &mockPlayerState{blockID: "test-block", playerID: "test-player"}

	// Test no selection - should not error but mark as incomplete with 1 attempt
	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	assert.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Verify player data shows 1 attempt with no selections
	var playerData QuizPlayerData
	err = json.Unmarshal(newState.GetPlayerData(), &playerData)
	require.NoError(t, err)
	assert.Equal(t, 1, playerData.Attempts)
	assert.Empty(t, playerData.SelectedOptions)
	assert.False(t, playerData.IsCorrect)

	// Test correct answer (use fresh state)
	freshState := &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_1"}}
	newState, err = block.ValidatePlayerInput(freshState, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 100, newState.GetPointsAwarded())

	// Verify player data
	err = json.Unmarshal(newState.GetPlayerData(), &playerData)
	require.NoError(t, err)
	assert.Equal(t, []string{"option_1"}, playerData.SelectedOptions)
	assert.Equal(t, 1, playerData.Attempts)
	assert.True(t, playerData.IsCorrect)

	// Test incorrect answer
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_0"}}
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	err = json.Unmarshal(newState.GetPlayerData(), &playerData)
	require.NoError(t, err)
	assert.Equal(t, []string{"option_0"}, playerData.SelectedOptions)
	assert.False(t, playerData.IsCorrect)
}

func TestQuizBlock_ValidatePlayerInput_MultipleChoice(t *testing.T) {
	block := QuizBlock{
		BaseBlock: BaseBlock{
			Points: 100,
		},
		Question:       "Select all programming languages:",
		MultipleChoice: true,
		RetryEnabled:   false,
		Options: []QuizOption{
			{ID: "option_0", Text: "Python", IsCorrect: true},
			{ID: "option_1", Text: "HTML", IsCorrect: false},
			{ID: "option_2", Text: "JavaScript", IsCorrect: true},
			{ID: "option_3", Text: "CSS", IsCorrect: false},
		},
	}

	state := &mockPlayerState{blockID: "test-block", playerID: "test-player"}

	// Test perfect answer (all correct options, no incorrect)
	input := map[string][]string{"quiz_option": {"option_0", "option_2"}}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 100, newState.GetPointsAwarded())

	var playerData QuizPlayerData
	err = json.Unmarshal(newState.GetPlayerData(), &playerData)
	require.NoError(t, err)
	assert.True(t, playerData.IsCorrect)

	// Test partial answer (some correct, some incorrect)
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_0", "option_1"}} // Python (correct) + HTML (incorrect)
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())

	// Should get partial points: 2 out of 4 correct = round(100 * 0.50) = 50
	// Python selected (correct), HTML selected (incorrect), JavaScript not selected (incorrect), CSS not selected (correct)
	assert.Equal(t, 50, newState.GetPointsAwarded())

	err = json.Unmarshal(newState.GetPlayerData(), &playerData)
	require.NoError(t, err)
	assert.False(t, playerData.IsCorrect)

	// Test only correct answers but not all
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_0"}} // Only Python
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())

	// Should get partial points: 3 out of 4 correct = round(100 * 0.75) = 75
	// Python selected (correct), HTML not selected (correct), JavaScript not selected (incorrect), CSS not selected (correct)
	assert.Equal(t, 75, newState.GetPointsAwarded())

	// Test all incorrect selections
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_1", "option_3"}} // HTML and CSS (both incorrect)
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	// Should get 0 points: 0 out of 4 correct = round(100 * 0.0) = 0
	// Python not selected (incorrect), HTML selected (incorrect), JavaScript not selected (incorrect), CSS selected (incorrect)
	assert.Equal(t, 0, newState.GetPointsAwarded())
}

func TestQuizBlock_ValidatePlayerInput_RetryEnabled(t *testing.T) {
	// Test single choice with retry
	block := QuizBlock{
		BaseBlock: BaseBlock{
			Points: 100,
		},
		Question:       "What is 2+2?",
		MultipleChoice: false,
		RetryEnabled:   true,
		Options: []QuizOption{
			{ID: "option_0", Text: "3", IsCorrect: false},
			{ID: "option_1", Text: "4", IsCorrect: true},
		},
	}

	state := &mockPlayerState{blockID: "test-block", playerID: "test-player"}

	// Test incorrect answer with retry enabled (single choice)
	input := map[string][]string{"quiz_option": {"option_0"}}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete()) // Should remain incomplete for retry
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test correct answer with retry enabled (single choice)
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_1"}}
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 100, newState.GetPointsAwarded())

	// Test multiple choice with retry
	multiBlock := QuizBlock{
		BaseBlock: BaseBlock{
			Points: 100,
		},
		Question:       "Select programming languages:",
		MultipleChoice: true,
		RetryEnabled:   true,
		Options: []QuizOption{
			{ID: "option_0", Text: "Python", IsCorrect: true},
			{ID: "option_1", Text: "HTML", IsCorrect: false},
			{ID: "option_2", Text: "JavaScript", IsCorrect: true},
			{ID: "option_3", Text: "CSS", IsCorrect: false},
		},
	}

	// Test partial answer with multiple choice retry - should get partial points but remain incomplete
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_0"}} // Only Python (partial answer)
	newState, err = multiBlock.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())           // Should remain incomplete for retry
	assert.Equal(t, 75, newState.GetPointsAwarded()) // 3 out of 4 correct = 75 points

	// Test perfect answer with multiple choice retry - should complete
	state = &mockPlayerState{blockID: "test-block", playerID: "test-player"}
	input = map[string][]string{"quiz_option": {"option_0", "option_2"}} // Perfect answer
	newState, err = multiBlock.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete()) // Should complete
	assert.Equal(t, 100, newState.GetPointsAwarded())
}

func TestQuizBlock_CalculatePoints_EdgeCases(t *testing.T) {
	// Test with no correct options
	block := QuizBlock{
		BaseBlock: BaseBlock{Points: 100},
		Options: []QuizOption{
			{ID: "option_0", Text: "A", IsCorrect: false},
			{ID: "option_1", Text: "B", IsCorrect: false},
		},
	}

	points, isCorrect := block.calculatePoints([]string{"option_0"})
	assert.Equal(t, 0, points)
	assert.False(t, isCorrect)

	// Test with empty options
	block.Options = []QuizOption{}
	points, isCorrect = block.calculatePoints([]string{"option_0"})
	assert.Equal(t, 0, points)
	assert.False(t, isCorrect)
}

func TestQuizBlock_GetShuffledOptions(t *testing.T) {
	block := QuizBlock{
		RandomizeOrder: true,
		Options: []QuizOption{
			{ID: "option_0", Text: "A", IsCorrect: false},
			{ID: "option_1", Text: "B", IsCorrect: true},
			{ID: "option_2", Text: "C", IsCorrect: false},
		},
	}

	// Test that shuffle returns same length
	shuffled := block.GetShuffledOptions()
	assert.Len(t, shuffled, 3)

	// Test that all original options are present (though possibly in different order)
	originalIDs := make(map[string]bool)
	shuffledIDs := make(map[string]bool)

	for _, option := range block.Options {
		originalIDs[option.ID] = true
	}
	for _, option := range shuffled {
		shuffledIDs[option.ID] = true
	}

	assert.Equal(t, originalIDs, shuffledIDs)

	// Test with randomization disabled
	block.RandomizeOrder = false
	notShuffled := block.GetShuffledOptions()
	assert.Equal(t, block.Options, notShuffled)

	// Test with single option (should not shuffle)
	block.RandomizeOrder = true
	block.Options = []QuizOption{{ID: "option_0", Text: "A", IsCorrect: true}}
	single := block.GetShuffledOptions()
	assert.Equal(t, block.Options, single)
}

func TestNewQuizBlock(t *testing.T) {
	base := BaseBlock{
		ID:         "test-id",
		LocationID: "location-123",
		Type:       "quiz_block",
		Order:      1,
		Points:     50,
	}

	block := NewQuizBlock(base)

	assert.Equal(t, base, block.BaseBlock)
	assert.Equal(t, "", block.Question)
	assert.Empty(t, block.Options)
	assert.False(t, block.MultipleChoice)
	assert.False(t, block.RandomizeOrder)
	assert.False(t, block.RetryEnabled)
}

