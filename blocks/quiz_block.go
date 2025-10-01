package blocks

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// QuizBlock allows players to answer multiple choice questions.
type QuizBlock struct {
	BaseBlock
	Question       string       `json:"question"`        // Markdown question text
	Options        []QuizOption `json:"options"`         // Answer choices
	MultipleChoice bool         `json:"multiple_choice"` // Whether multiple answers are allowed
	RandomizeOrder bool         `json:"randomize_order"` // Shuffle options
	RetryEnabled   bool         `json:"retry_enabled"`   // Allow players to retry
}

// QuizOption represents an individual answer choice.
type QuizOption struct {
	ID        string `json:"id"`         // Unique identifier
	Text      string `json:"text"`       // Markdown answer text
	IsCorrect bool   `json:"is_correct"` // Whether this option is correct
	Order     int    `json:"order"`      // Display order
}

// QuizPlayerData stores player progress.
type QuizPlayerData struct {
	SelectedOptions []string `json:"selected_options"` // List of selected option IDs
	Attempts        int      `json:"attempts"`         // Number of submission attempts
	IsCorrect       bool     `json:"is_correct"`       // Whether answer is correct
}

// Basic Attribute Getters.
func (b *QuizBlock) GetName() string { return "Quiz" }

func (b *QuizBlock) GetDescription() string {
	return "Answer a quiz question with multiple choice options."
}

func (b *QuizBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-message-circle-question-mark-icon lucide-message-circle-question-mark"><path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>`
}

func (b *QuizBlock) GetType() string { return "quiz_block" }

func (b *QuizBlock) GetID() string { return b.ID }

func (b *QuizBlock) GetLocationID() string { return b.LocationID }

func (b *QuizBlock) GetOrder() int { return b.Order }

func (b *QuizBlock) GetPoints() int { return b.Points }

func (b *QuizBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data operations.
func (b *QuizBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *QuizBlock) UpdateBlockData(input map[string][]string) error {
	// Parse points
	pointsInput, ok := input["points"]
	if ok && len(pointsInput[0]) > 0 {
		points, err := strconv.Atoi(pointsInput[0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	} else {
		b.Points = 0
	}

	// Update question
	if question, exists := input["question"]; exists && len(question) > 0 {
		b.Question = question[0]
	}

	// Update multiple choice setting
	b.MultipleChoice = false
	if multipleChoice, exists := input["multiple_choice"]; exists && len(multipleChoice) > 0 {
		b.MultipleChoice = multipleChoice[0] == "on"
	}

	// Update randomize order setting
	b.RandomizeOrder = false
	if randomizeOrder, exists := input["randomize_order"]; exists && len(randomizeOrder) > 0 {
		b.RandomizeOrder = randomizeOrder[0] == "on"
	}

	// Update retry enabled setting
	b.RetryEnabled = false
	if retryEnabled, exists := input["retry_enabled"]; exists && len(retryEnabled) > 0 {
		b.RetryEnabled = retryEnabled[0] == "on"
	}

	// Process options
	b.Options = []QuizOption{}
	optionTexts := input["option_text"]
	optionCorrect := input["option_correct"]

	for i, text := range optionTexts {
		if strings.TrimSpace(text) == "" {
			continue // Skip empty options
		}

		option := QuizOption{
			ID:        fmt.Sprintf("option_%d", i),
			Text:      text,
			IsCorrect: false,
			Order:     i,
		}

		// Check if this option is marked as correct
		for _, correctValue := range optionCorrect {
			if correctValue == fmt.Sprintf("option_%d", i) {
				option.IsCorrect = true
				break
			}
		}

		b.Options = append(b.Options, option)
	}

	// Validate that at least one option is marked as correct
	hasCorrectOption := false
	for _, option := range b.Options {
		if option.IsCorrect {
			hasCorrectOption = true
			break
		}
	}
	if len(b.Options) > 0 && !hasCorrectOption {
		return errors.New("at least one option must be marked as correct")
	}

	return nil
}

// Validation and Points Calculation.
func (b *QuizBlock) RequiresValidation() bool {
	return true
}

func (b *QuizBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	newState := state

	// Parse current player data if it exists
	var playerData QuizPlayerData
	if state.GetPlayerData() != nil {
		if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
			return state, fmt.Errorf("failed to parse player data: %w", err)
		}
	}

	// Get player's selected options from input
	selectedOptions, exists := input["quiz_option"]
	if !exists || len(selectedOptions) == 0 {
		// For no selection, return the current state without changes but mark as having an attempt
		var playerData QuizPlayerData
		if state.GetPlayerData() != nil {
			if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
				return state, fmt.Errorf("failed to parse player data: %w", err)
			}
		}
		playerData.Attempts++
		playerData.SelectedOptions = []string{}
		playerData.IsCorrect = false

		newPlayerData, err := json.Marshal(playerData)
		if err != nil {
			return state, fmt.Errorf("failed to save player data: %w", err)
		}
		newState.SetPlayerData(newPlayerData)
		newState.SetComplete(false)
		newState.SetPointsAwarded(0)
		return newState, nil
	}

	// Store the player's selections and increment attempts
	playerData.SelectedOptions = selectedOptions
	playerData.Attempts++

	// Calculate points and correctness
	points, isCorrect := b.calculatePoints(playerData.SelectedOptions)
	playerData.IsCorrect = isCorrect

	// Marshal the updated player data
	newPlayerData, err := json.Marshal(playerData)
	if err != nil {
		return state, fmt.Errorf("failed to save player data: %w", err)
	}
	newState.SetPlayerData(newPlayerData)

	// Mark as complete and award points
	if b.RetryEnabled {
		// For retry-enabled blocks, only mark complete if perfect score
		if isCorrect {
			newState.SetComplete(true)
			newState.SetPointsAwarded(points)
		} else {
			newState.SetComplete(false)
			// Award partial points for multiple choice even when not complete
			if b.MultipleChoice {
				newState.SetPointsAwarded(points)
			} else {
				newState.SetPointsAwarded(0)
			}
		}
	} else {
		// For non-retry blocks, always complete and award calculated points
		newState.SetComplete(true)
		newState.SetPointsAwarded(points)
	}

	return newState, nil
}

// calculatePoints calculates points based on selected options.
func (b *QuizBlock) calculatePoints(selectedOptions []string) (int, bool) {
	if len(b.Options) == 0 {
		return 0, false
	}

	correctOptions := make(map[string]bool)
	totalCorrect := 0
	for _, option := range b.Options {
		if option.IsCorrect {
			correctOptions[option.ID] = true
			totalCorrect++
		}
	}

	if totalCorrect == 0 {
		return 0, false
	}

	if b.MultipleChoice {
		// Multiple choice: proportional scoring based on correct/incorrect answers
		correctAnswers := 0

		// Count how many options are answered correctly (checked if should be checked, unchecked if should be unchecked)
		selectedSet := make(map[string]bool)
		for _, selectedID := range selectedOptions {
			selectedSet[selectedID] = true
		}

		for _, option := range b.Options {
			// Correct if: (option is correct AND selected) OR (option is incorrect AND not selected)
			if (option.IsCorrect && selectedSet[option.ID]) || (!option.IsCorrect && !selectedSet[option.ID]) {
				correctAnswers++
			}
		}

		// Calculate proportional points: round(points * (correct/total))
		ratio := float64(correctAnswers) / float64(len(b.Options))
		points := int(float64(b.Points)*ratio + 0.5) // +0.5 for rounding

		// Perfect score means all correct
		isCorrect := correctAnswers == len(b.Options)

		return points, isCorrect
	} else {
		// Single choice: all or nothing
		if len(selectedOptions) == 1 && correctOptions[selectedOptions[0]] {
			return b.Points, true
		}
		return 0, false
	}
}

// shuffleOptions returns a shuffled copy of the options slice.
func (b *QuizBlock) shuffleOptions() []QuizOption {
	if !b.RandomizeOrder || len(b.Options) <= 1 {
		return b.Options
	}

	shuffled := make([]QuizOption, len(b.Options))
	copy(shuffled, b.Options)

	// Fisher-Yates shuffle
	for i := len(shuffled) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		shuffled[i], shuffled[j.Int64()] = shuffled[j.Int64()], shuffled[i]
	}

	return shuffled
}

// GetShuffledOptions returns the options in shuffled order if randomization is enabled.
func (b *QuizBlock) GetShuffledOptions() []QuizOption {
	return b.shuffleOptions()
}

// NewQuizBlock creates a new quiz block instance.
func NewQuizBlock(base BaseBlock) *QuizBlock {
	return &QuizBlock{
		BaseBlock:      base,
		Question:       "",
		Options:        []QuizOption{},
		MultipleChoice: false,
		RandomizeOrder: false,
		RetryEnabled:   false,
	}
}
