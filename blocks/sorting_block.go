package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strconv"

	"github.com/google/uuid"
)

// SortingBlock is a quiz-type blcok that requires players to sort items in a specific order.
type SortingBlock struct {
	BaseBlock
	Content       string        `json:"content"`
	Items         []SortingItem `json:"items"`
	ScoringScheme string        `json:"scoring_scheme"`
}

// SortingItem represents an individual item to be sorted.
type SortingItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Position    int    `json:"position"` // The correct position (1-based)
}

// Scoring schemes.
const (
	AllOrNothing            = "all_or_nothing"
	CorrectItemCorrectPlace = "correct_item_correct_place"
	RetryUntilCorrect       = "retry_until_correct"
)

// SortingPlayerData stores player progress.
type SortingPlayerData struct {
	PlayerOrder  []string `json:"player_order"`  // List of item IDs in player's submitted order
	ShuffleOrder []string `json:"shuffle_order"` // Shuffled order shown to player initially
	Attempts     int      `json:"attempts"`      // Number of attempts made so far
	IsCorrect    bool     `json:"is_correct"`    // Whether the current order is correct
}

// GetName returns the block type name.
func (b *SortingBlock) GetName() string { return "Sorting" }

func (b *SortingBlock) GetDescription() string {
	return "Sort items in the correct order."
}

func (b *SortingBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-arrow-down-wide-narrow"><path d="m3 16 4 4 4-4"/><path d="M7 20V4"/><path d="M11 4h10"/><path d="M11 8h7"/><path d="M11 12h4"/></svg>`
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

// ParseData parses the block data from JSON.
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

// RequiresValidation returns whether this block requires player input validation.
func (b *SortingBlock) RequiresValidation() bool { return true }

func (b *SortingBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	playerData, err := b.parsePlayerData(state)
	if err != nil {
		return state, err
	}

	// If the player already has a correct solution in RetryUntilCorrect mode, don't process further
	if b.ScoringScheme == RetryUntilCorrect && playerData.IsCorrect {
		return state, nil
	}

	itemOrder, err := b.getItemOrder(input)
	if err != nil {
		return state, err
	}

	playerData = b.updatePlayerData(playerData, itemOrder, state)
	isCorrect := playerData.IsCorrect

	newState, err := b.savePlayerData(state, playerData)
	if err != nil {
		return state, err
	}

	b.applyScoring(&newState, isCorrect, itemOrder)
	return newState, nil
}

func (b *SortingBlock) parsePlayerData(state PlayerState) (SortingPlayerData, error) {
	var playerData SortingPlayerData
	if state.GetPlayerData() != nil {
		if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
			return playerData, fmt.Errorf("failed to parse player data: %w", err)
		}
	}
	return playerData, nil
}

func (b *SortingBlock) getItemOrder(input map[string][]string) ([]string, error) {
	itemOrder, exists := input["sorting-item-order"]
	if !exists || len(itemOrder) == 0 {
		return nil, errors.New("sorting order is required")
	}
	return itemOrder, nil
}

func (b *SortingBlock) updatePlayerData(
	playerData SortingPlayerData,
	itemOrder []string,
	state PlayerState,
) SortingPlayerData {
	// Initialize shuffle order if it doesn't exist
	if len(playerData.ShuffleOrder) == 0 {
		allItemIDs := make([]string, len(b.Items))
		for i, item := range b.Items {
			allItemIDs[i] = item.ID
		}
		playerData.ShuffleOrder = deterministicShuffle(allItemIDs, state.GetBlockID()+state.GetPlayerID())
	}

	playerData.PlayerOrder = itemOrder
	playerData.Attempts++
	playerData.IsCorrect = b.orderIsCorrect(itemOrder)

	return playerData
}

func (b *SortingBlock) savePlayerData(state PlayerState, playerData SortingPlayerData) (PlayerState, error) {
	newPlayerData, err := json.Marshal(playerData)
	if err != nil {
		return state, fmt.Errorf("failed to save player data: %w", err)
	}
	state.SetPlayerData(newPlayerData)
	return state, nil
}

func (b *SortingBlock) applyScoring(state *PlayerState, isCorrect bool, itemOrder []string) {
	switch b.ScoringScheme {
	case RetryUntilCorrect:
		b.applyRetryUntilCorrectScoring(state, isCorrect)
	case AllOrNothing:
		b.applyAllOrNothingScoring(state, isCorrect)
	case CorrectItemCorrectPlace:
		b.applyCorrectItemCorrectPlaceScoring(state, itemOrder)
	default:
		b.applyAllOrNothingScoring(state, isCorrect)
	}
}

func (b *SortingBlock) applyRetryUntilCorrectScoring(state *PlayerState, isCorrect bool) {
	if isCorrect {
		(*state).SetComplete(true)
		(*state).SetPointsAwarded(b.Points)
	} else {
		(*state).SetComplete(false)
		(*state).SetPointsAwarded(0)
	}
}

func (b *SortingBlock) applyAllOrNothingScoring(state *PlayerState, isCorrect bool) {
	(*state).SetComplete(true)
	if isCorrect {
		(*state).SetPointsAwarded(b.Points)
	} else {
		(*state).SetPointsAwarded(0)
	}
}

func (b *SortingBlock) applyCorrectItemCorrectPlaceScoring(state *PlayerState, itemOrder []string) {
	(*state).SetComplete(true)
	(*state).SetPointsAwarded(b.calculateCorrectItemCorrectPlacePoints(itemOrder))
}

// orderIsCorrect checks if the submitted order perfectly matches the expected order.
func (b *SortingBlock) orderIsCorrect(playerOrder []string) bool {
	if len(playerOrder) != len(b.Items) {
		return false
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
			return false
		}
	}

	return true
}

// calculateCorrectItemCorrectPlacePoints awards points for each correctly placed item.
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

// deterministicShuffle creates a consistent shuffle of items based on a seed string
// This ensures the same player always sees the same shuffle for a given block.
func deterministicShuffle(items []string, seed string) []string {
	// Create a copy of the original items
	result := make([]string, len(items))
	copy(result, items)

	// Create a deterministic random number generator
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	//nolint:gosec // Deterministic shuffle for game consistency, not cryptographic use
	r := rand.New(rand.NewPCG(uint64(h.Sum32()), 0))

	// Shuffle the items using Fisher-Yates algorithm
	for i := len(result) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return result
}
