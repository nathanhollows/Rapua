package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"

	"github.com/google/uuid"
)

// SortingBlock allows players to sort items in the correct order
type SortingBlock struct {
	BaseBlock
	Content       string        `json:"content"`
	Items         []SortingItem `json:"items"`
	ScoringScheme string        `json:"scoring_scheme"`
}

// SortingItem represents an individual item to be sorted
type SortingItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Position    int    `json:"position"` // The correct position (1-based)
}

// Scoring schemes
const (
	AllOrNothing            = "all_or_nothing"
	CorrectItemCorrectPlace = "correct_item_correct_place"
	RetryUntilCorrect       = "retry_until_correct"
)

// SortingPlayerData stores player progress
type SortingPlayerData struct {
	PlayerOrder []string `json:"player_order"` // List of item IDs in player's submitted order
	ShuffleOrder []string `json:"shuffle_order"` // Shuffled order shown to player initially
	Attempts    int      `json:"attempts"`     // Number of attempts made so far
	IsCorrect   bool     `json:"is_correct"`   // Whether the current order is correct
}

// Basic Attributes Getters
func (b *SortingBlock) GetName() string { return "Sorting" }

func (b *SortingBlock) GetDescription() string {
	return "Sort items in the correct order."
}

func (b *SortingBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-arrow-down-wide-narrow-icon lucide-arrow-down-wide-narrow"><path d="m3 16 4 4 4-4"/><path d="M7 20V4"/><path d="M11 4h10"/><path d="M11 8h7"/><path d="M11 12h4"/></svg>`
}

func (b *SortingBlock) GetType() string { return "sorting" }

func (b *SortingBlock) GetID() string { return b.ID }

func (b *SortingBlock) GetLocationID() string { return b.LocationID }

func (b *SortingBlock) GetOrder() int { return b.Order }

func (b *SortingBlock) GetPoints() int { return b.Points }

func (b *SortingBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations
func (b *SortingBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *SortingBlock) UpdateBlockData(input map[string][]string) error {
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

	// Update content
	if content, exists := input["content"]; exists && len(content) > 0 {
		b.Content = content[0]
	}

	// Parse scoring scheme
	if scheme, exists := input["scoring_scheme"]; exists && len(scheme) > 0 {
		b.ScoringScheme = scheme[0]
	} else {
		b.ScoringScheme = AllOrNothing
	}

	// No longer need to parse scoring percentage as we're using fixed proportion of total points

	// Update sorting items
	itemDescriptions := input["sorting-items"]
	itemIDs := input["sorting-item-ids"]

	updatedItems := make([]SortingItem, 0, len(itemDescriptions))
	for i, desc := range itemDescriptions {
		if desc == "" {
			continue
		}

		var id string
		if i < len(itemIDs) && itemIDs[i] != "" {
			id = itemIDs[i]
		} else {
			uuid, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("failed to generate UUID: %w", err)
			}
			id = uuid.String()
		}

		updatedItems = append(updatedItems, SortingItem{
			ID:          id,
			Description: desc,
			Position:    i + 1, // Position is 1-based
		})
	}
	b.Items = updatedItems
	return nil
}

// Validation and Points Calculation
func (b *SortingBlock) RequiresValidation() bool { return true }

func (b *SortingBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	newState := state

	// Parse player data from the existing state
	var playerData SortingPlayerData
	if state.GetPlayerData() != nil {
		if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
			return state, fmt.Errorf("failed to parse player data: %w", err)
		}
	}

	// If the player already has a correct solution in RetryUntilCorrect mode, don't process further
	if b.ScoringScheme == RetryUntilCorrect && playerData.IsCorrect && state.IsComplete() {
		return state, nil
	}

	// Get player's ordering from input
	itemOrder, exists := input["sorting-item-order"]
	if !exists || len(itemOrder) == 0 {
		return state, errors.New("sorting order is required")
	}

	// Always generate a consistent shuffle order for the player
	// Create a deterministic shuffled order based on the blockID and playerID
	// This ensures the same player always sees the same shuffle for a given block
	if len(playerData.ShuffleOrder) == 0 {
		// Generate an array of all item IDs for first-time shuffling
		allItemIDs := make([]string, len(b.Items))
		for i, item := range b.Items {
			allItemIDs[i] = item.ID
		}
		playerData.ShuffleOrder = deterministicShuffle(allItemIDs, state.GetBlockID()+state.GetPlayerID())
	}

	// Update player data with new ordering
	playerData.PlayerOrder = itemOrder
	playerData.Attempts++

	// Calculate points based on scoring scheme
	points := b.calculatePoints(playerData.PlayerOrder)

	// Check if order is completely correct
	perfectOrder := b.calculateAllOrNothingPoints(playerData.PlayerOrder) > 0
	playerData.IsCorrect = perfectOrder

	// Marshal the updated player data
	newPlayerData, err := json.Marshal(playerData)
	if err != nil {
		return state, fmt.Errorf("failed to save player data: %w", err)
	}
	newState.SetPlayerData(newPlayerData)

	// Handle different scoring schemes for completion status
	switch b.ScoringScheme {
	case RetryUntilCorrect:
		// For RetryUntilCorrect, only mark as complete when correct
		if perfectOrder {
			newState.SetComplete(true)
			newState.SetPointsAwarded(b.Points) // Award full points
		} else {
			newState.SetComplete(false)
			newState.SetPointsAwarded(0)
		}
	case AllOrNothing:
		// For AllOrNothing, mark as complete only if perfect
		if perfectOrder {
			newState.SetComplete(true)
			newState.SetPointsAwarded(b.Points) // Award full points
		} else {
			newState.SetComplete(true) // Still mark as complete (one shot)
			newState.SetPointsAwarded(0) // But award no points
		}
	default:
		// For other scoring schemes (CorrectItemCorrectPlace, ConsecutiveRuns)
		// Mark as complete and award proportional points
		newState.SetComplete(true)
		newState.SetPointsAwarded(points)
	}

	return newState, nil
}

// calculatePoints determines how many points to award based on the scoring scheme
func (b *SortingBlock) calculatePoints(playerOrder []string) int {
	switch b.ScoringScheme {
	case AllOrNothing:
		return b.calculateAllOrNothingPoints(playerOrder)
	case CorrectItemCorrectPlace:
		return b.calculateCorrectItemCorrectPlacePoints(playerOrder)
	case RetryUntilCorrect:
		return b.calculateRetryUntilCorrectPoints(playerOrder)
	default:
		return b.calculateAllOrNothingPoints(playerOrder)
	}
}

// calculateAllOrNothingPoints awards full points only if entire sequence is correct
func (b *SortingBlock) calculateAllOrNothingPoints(playerOrder []string) int {
	if len(playerOrder) != len(b.Items) {
		return 0
	}

	// Create a map for quick lookup of correct positions
	itemPositions := make(map[string]int)
	for _, item := range b.Items {
		itemPositions[item.ID] = item.Position
	}

	// Check if player order matches correct positions
	for i, itemID := range playerOrder {
		correctPos, exists := itemPositions[itemID]
		if !exists || correctPos != i+1 {
			return 0
		}
	}

	return b.Points
}

// calculateCorrectItemCorrectPlacePoints awards points for each correctly placed item
func (b *SortingBlock) calculateCorrectItemCorrectPlacePoints(playerOrder []string) int {
	if len(playerOrder) != len(b.Items) {
		return 0
	}

	// Create a map for quick lookup of correct positions
	itemPositions := make(map[string]int)
	for _, item := range b.Items {
		itemPositions[item.ID] = item.Position
	}

	// Count correct placements
	correctPlacements := 0
	for i, itemID := range playerOrder {
		correctPos, exists := itemPositions[itemID]
		if exists && correctPos == i+1 {
			correctPlacements++
		}
	}

	// Calculate points based on proportion of correct placements
	if correctPlacements == 0 {
		return 0
	}

	// Award points proportionally to the number of correct placements
	pointsPercentage := float64(correctPlacements) / float64(len(b.Items))
	return int(float64(b.Points) * pointsPercentage)
}


// calculateRetryUntilCorrectPoints checks if the order is correct and returns points only if it is
// This function is meant to be used with the retry until correct scoring scheme
// It returns 0 for incorrect answers, allowing multiple attempts
func (b *SortingBlock) calculateRetryUntilCorrectPoints(playerOrder []string) int {
	if len(playerOrder) != len(b.Items) {
		return 0
	}

	// Create a map for quick lookup of correct positions
	itemPositions := make(map[string]int)
	for _, item := range b.Items {
		itemPositions[item.ID] = item.Position
	}

	// Check if player order matches correct positions
	for i, itemID := range playerOrder {
		correctPos, exists := itemPositions[itemID]
		if !exists || correctPos != i+1 {
			return 0 // Order is incorrect, return 0 points
		}
	}

	// Order is correct, award full points
	return b.Points
}

// deterministicShuffle creates a consistent shuffle of items based on a seed string
// This ensures the same player always sees the same shuffle for a given block
func deterministicShuffle(items []string, seed string) []string {
	// Create a copy of the original items
	result := make([]string, len(items))
	copy(result, items)
	
	// Create a deterministic random number generator
	h := fnv.New32a()
	h.Write([]byte(seed))
	r := rand.New(rand.NewSource(int64(h.Sum32())))
	
	// Shuffle the items using Fisher-Yates algorithm
	for i := len(result) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}
	
	return result
}