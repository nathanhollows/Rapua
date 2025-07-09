package blocks

import (
	"encoding/json"
	"errors"
	"strconv"
)

type ClueBlock struct {
	BaseBlock
	ClueText        string `json:"clue_text"`
	DescriptionText string `json:"description_text"`
	ButtonLabel     string `json:"button_label"`
}

type clueBlockData struct {
	IsRevealed bool `json:"is_revealed"`
}

// Basic Attributes Getters

func (b *ClueBlock) GetID() string         { return b.ID }
func (b *ClueBlock) GetType() string       { return "clue" }
func (b *ClueBlock) GetLocationID() string { return b.LocationID }
func (b *ClueBlock) GetName() string       { return "Clue" }
func (b *ClueBlock) GetDescription() string {
	return "Players can reveal a clue by spending points."
}
func (b *ClueBlock) GetOrder() int  { return b.Order }
func (b *ClueBlock) GetPoints() int { return b.Points }
func (b *ClueBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-lightbulb"><path d="M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.7 1.3 1.5 1.5 2.5"/><path d="M9 18h6"/><path d="M10 22h4"/></svg>`
}
func (b *ClueBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *ClueBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *ClueBlock) UpdateBlockData(input map[string][]string) error {
	// Points (standard points field)
	if pointsInput, ok := input["points"]; ok && len(pointsInput) > 0 && pointsInput[0] != "" {
		points, err := strconv.Atoi(pointsInput[0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		if points > 0 {
			points = -points // Points are negative for cost
		}
		b.Points = points
	} else {
		b.Points = 0
	}

	// Clue text (markdown content)
	if clueText, exists := input["clue_text"]; exists && len(clueText) > 0 {
		b.ClueText = clueText[0]
	}

	// Description text (markdown content)
	if descriptionText, exists := input["description_text"]; exists && len(descriptionText) > 0 {
		b.DescriptionText = descriptionText[0]
	}

	// Button label
	if buttonLabel, exists := input["button_label"]; exists && len(buttonLabel) > 0 && buttonLabel[0] != "" {
		b.ButtonLabel = buttonLabel[0]
	} else {
		b.ButtonLabel = "Reveal Clue"
	}

	return nil
}

// Validation and Points Calculation

func (b *ClueBlock) RequiresValidation() bool { return true }

func (b *ClueBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	newState := state

	// Parse current player data if it exists
	var playerData clueBlockData
	if state.GetPlayerData() != nil {
		if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
			return state, errors.New("failed to parse player data")
		}
	}

	// Check if the player is trying to reveal the clue
	if revealInput, exists := input["reveal_clue"]; exists && len(revealInput) > 0 && revealInput[0] == "true" {
		// Mark the clue as revealed
		playerData.IsRevealed = true

		// Update player data
		newPlayerData, err := json.Marshal(playerData)
		if err != nil {
			return state, errors.New("failed to save player data")
		}
		newState.SetPlayerData(newPlayerData)

		// Mark as complete and award the points (which should be negative for cost)
		newState.SetComplete(true)
		newState.SetPointsAwarded(b.Points)
	}

	return newState, nil
}
