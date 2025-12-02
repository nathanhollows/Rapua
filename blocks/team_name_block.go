package blocks

import (
	"encoding/json"
	"errors"
	"strconv"
)

type TeamNameChangerBlock struct {
	BaseBlock
	ButtonText    string `json:"button_text"`
	AllowChanging bool   `json:"allow_changing"`
}

// Basic Attributes Getters

func (b *TeamNameChangerBlock) GetID() string         { return b.ID }
func (b *TeamNameChangerBlock) GetType() string       { return "team_name" }
func (b *TeamNameChangerBlock) GetLocationID() string { return b.LocationID }
func (b *TeamNameChangerBlock) GetName() string       { return "Team Name" }
func (b *TeamNameChangerBlock) GetDescription() string {
	return "Allow players to set or change their team name."
}
func (b *TeamNameChangerBlock) GetOrder() int  { return b.Order }
func (b *TeamNameChangerBlock) GetPoints() int { return b.Points }
func (b *TeamNameChangerBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-signature-icon lucide-signature"><path d="m21 17-2.156-1.868A.5.5 0 0 0 18 15.5v.5a1 1 0 0 1-1 1h-2a1 1 0 0 1-1-1c0-2.545-3.991-3.97-8.5-4a1 1 0 0 0 0 5c4.153 0 4.745-11.295 5.708-13.5a2.5 2.5 0 1 1 3.31 3.284"/><path d="M3 21h18"/></svg>`
}
func (b *TeamNameChangerBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *TeamNameChangerBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *TeamNameChangerBlock) UpdateBlockData(input map[string][]string) error {
	// Points
	if pointsInput, ok := input["points"]; ok && len(pointsInput) > 0 && pointsInput[0] != "" {
		points, err := strconv.Atoi(pointsInput[0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	}

	if buttonText, exists := input["button_text"]; exists && len(buttonText) > 0 {
		b.ButtonText = buttonText[0]
	}
	// Checkbox: if present in form data, it's checked; if absent, it's unchecked
	if allowChanging, exists := input["allow_changing"]; exists && len(allowChanging) > 0 {
		b.AllowChanging = allowChanging[0] == "true" || allowChanging[0] == "on"
	} else {
		b.AllowChanging = false
	}
	return nil
}

// Validation and Points Calculation

func (b *TeamNameChangerBlock) RequiresValidation() bool {
	return true
}

func (b *TeamNameChangerBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// I don't care about the input; just mark the block as complete when the player interacts with it
	// The actual input is handled elsewhere via HTMX since it affects team state and blocks can't access that directly
	state.SetComplete(true)
	state.SetPointsAwarded(b.Points)
	return state, nil
}
