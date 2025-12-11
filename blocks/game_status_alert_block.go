package blocks

import (
	"encoding/json"
	"strconv"
)

type GameStatusAlertBlock struct {
	BaseBlock
	ClosedMessage    string `json:"closed_message"`
	ScheduledMessage string `json:"scheduled_message"`
	ShowCountdown    bool   `json:"show_countdown"`
}

// Basic Attributes Getters

func (b *GameStatusAlertBlock) GetID() string         { return b.ID }
func (b *GameStatusAlertBlock) GetType() string       { return "game_status_alert" }
func (b *GameStatusAlertBlock) GetLocationID() string { return b.LocationID }
func (b *GameStatusAlertBlock) GetName() string       { return "Game Status" }
func (b *GameStatusAlertBlock) GetDescription() string {
	return "Display game status as an alert with optional countdown timer."
}
func (b *GameStatusAlertBlock) GetOrder() int  { return b.Order }
func (b *GameStatusAlertBlock) GetPoints() int { return b.Points }
func (b *GameStatusAlertBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-clock-alert-icon lucide-clock-alert"><path d="M12 6v6l4 2"/><path d="M20 12v5"/><path d="M20 21h.01"/><path d="M21.25 8.2A10 10 0 1 0 16 21.16"/></svg>`
}
func (b *GameStatusAlertBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *GameStatusAlertBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *GameStatusAlertBlock) UpdateBlockData(input map[string][]string) error {
	// Points
	if pointsInput, ok := input["points"]; ok && len(pointsInput) > 0 && pointsInput[0] != "" {
		points, err := strconv.Atoi(pointsInput[0])
		if err != nil {
			return err
		}
		b.Points = points
	}

	if closedMessage, exists := input["closed_message"]; exists && len(closedMessage) > 0 {
		b.ClosedMessage = closedMessage[0]
	}
	if scheduledMessage, exists := input["scheduled_message"]; exists && len(scheduledMessage) > 0 {
		b.ScheduledMessage = scheduledMessage[0]
	}

	// Checkboxes
	if showCountdown, exists := input["show_countdown"]; exists && len(showCountdown) > 0 {
		b.ShowCountdown = showCountdown[0] == FormValueTrue || showCountdown[0] == "on"
	} else {
		b.ShowCountdown = false
	}

	return nil
}

// Validation and Points Calculation

func (b *GameStatusAlertBlock) RequiresValidation() bool {
	return false
}

func (b *GameStatusAlertBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required; this is a display-only block
	state.SetComplete(true)
	return state, nil
}
