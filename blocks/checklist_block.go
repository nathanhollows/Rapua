package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

type ChecklistBlock struct {
	BaseBlock
	Content string          `json:"content"`
	List    []ChecklistItem `json:"list"`
}

type ChecklistItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Checked     bool   `json:"checked"`
}

func (c ChecklistItem) IsChecked(playerData json.RawMessage) bool {
	var player checklistPlayerData
	if err := json.Unmarshal(playerData, &player); err != nil {
		return false
	}

	for _, item := range player.CheckedItems {
		if item == c.ID {
			return true
		}
	}

	return false
}

// Unexported struct for storing player progress data in a block.
type checklistPlayerData struct {
	CheckedItems []string `json:"checked_items"`
}

// Ensure ChecklistBlock implements the Block interface

// GetName returns the block type name.
func (b *ChecklistBlock) GetName() string { return "Checklist" }

func (b *ChecklistBlock) GetDescription() string {
	return "Players must check off all items."
}

func (b *ChecklistBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-list-checks"><path d="m3 17 2 2 4-4"/><path d="m3 7 2 2 4-4"/><path d="M13 6h8"/><path d="M13 12h8"/><path d="M13 18h8"/></svg>`
}

func (b *ChecklistBlock) GetType() string { return "checklist" }

func (b *ChecklistBlock) GetID() string { return b.ID }

func (b *ChecklistBlock) GetLocationID() string { return b.LocationID }

func (b *ChecklistBlock) GetOrder() int { return b.Order }

func (b *ChecklistBlock) GetPoints() int { return b.Points }

func (b *ChecklistBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// ParseData parses the block data from JSON.
func (b *ChecklistBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *ChecklistBlock) UpdateBlockData(input map[string][]string) error {
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

	// Update checklist items
	itemDescriptions := input["checklist-items"]
	itemIDs := input["checklist-item-ids"]

	updatedList := make([]ChecklistItem, 0, len(itemDescriptions))
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
		updatedList = append(updatedList, ChecklistItem{
			ID:          id,
			Description: desc,
		})
	}
	b.List = updatedList
	return nil
}

// RequiresValidation returns whether this block requires player input validation.
func (b *ChecklistBlock) RequiresValidation() bool { return true }

func (b *ChecklistBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	newState := state

	// Parse player data from the existing state
	var playerData checklistPlayerData
	if state.GetPlayerData() != nil {
		err := json.Unmarshal(state.GetPlayerData(), &playerData)
		if err != nil {
			return state, fmt.Errorf("failed to parse player data: %w", err)
		}
	}

	// Update checked items based on the player's input
	playerData.CheckedItems = []string{}
	for _, item := range b.List {
		for _, inputItem := range input["checklist-item-ids"] {
			if item.ID == inputItem {
				playerData.CheckedItems = append(playerData.CheckedItems, item.ID)
			}
		}
	}

	// Marshal the updated player data back into the state
	newPlayerData, err := json.Marshal(playerData)
	if err != nil {
		return state, fmt.Errorf("failed to save player data: %w", err)
	}
	newState.SetPlayerData(newPlayerData)

	// Mark the newState as complete if all items are checked
	allChecked := len(playerData.CheckedItems) == len(b.List)
	if allChecked {
		newState.SetComplete(true)
		newState.SetPointsAwarded(b.Points)
	} else {
		newState.SetComplete(false)
		newState.SetPointsAwarded(0)
	}

	return newState, nil
}
