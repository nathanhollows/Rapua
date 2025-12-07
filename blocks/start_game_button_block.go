package blocks

import (
	"encoding/json"
	"strconv"
)

type StartGameButtonBlock struct {
	BaseBlock
	ScheduledButtonText string `json:"scheduled_button_text"`
	ActiveButtonText    string `json:"active_button_text"`
	ButtonStyle         string `json:"button_style"`
}

// Basic Attributes Getters

func (b *StartGameButtonBlock) GetID() string         { return b.ID }
func (b *StartGameButtonBlock) GetType() string       { return "start_game_button" }
func (b *StartGameButtonBlock) GetLocationID() string { return b.LocationID }
func (b *StartGameButtonBlock) GetName() string       { return "Start Button" }
func (b *StartGameButtonBlock) GetDescription() string {
	return "Display a button to start the game when active."
}
func (b *StartGameButtonBlock) GetOrder() int  { return b.Order }
func (b *StartGameButtonBlock) GetPoints() int { return b.Points }
func (b *StartGameButtonBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-play-circle"><circle cx="12" cy="12" r="10"/><polygon points="10 8 16 12 10 16 10 8"/></svg>`
}
func (b *StartGameButtonBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *StartGameButtonBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *StartGameButtonBlock) UpdateBlockData(input map[string][]string) error {
	// Points
	if pointsInput, ok := input["points"]; ok && len(pointsInput) > 0 && pointsInput[0] != "" {
		points, err := strconv.Atoi(pointsInput[0])
		if err != nil {
			return err
		}
		b.Points = points
	}

	if scheduledText, exists := input["scheduled_button_text"]; exists && len(scheduledText) > 0 {
		b.ScheduledButtonText = scheduledText[0]
	}
	if activeText, exists := input["active_button_text"]; exists && len(activeText) > 0 {
		b.ActiveButtonText = activeText[0]
	}
	if buttonStyle, exists := input["button_style"]; exists && len(buttonStyle) > 0 {
		b.ButtonStyle = buttonStyle[0]
	}

	return nil
}

// Validation and Points Calculation

func (b *StartGameButtonBlock) RequiresValidation() bool {
	return false
}

func (b *StartGameButtonBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required; this is a display-only block
	state.SetComplete(true)
	return state, nil
}

func (b *StartGameButtonBlock) GetButtonStyles() map[string]string {
	return map[string]string{
		"primary":   "Primary",
		"secondary": "Secondary",
		"accent":    "Accent",
		"neutral":   "Neutral",
	}
}
