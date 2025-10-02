package blocks

import (
	"encoding/json"
)

type DividerBlock struct {
	BaseBlock
	Title string `json:"title"`
}

// Basic Attributes Getters

func (b *DividerBlock) GetID() string         { return b.ID }
func (b *DividerBlock) GetType() string       { return "divider" }
func (b *DividerBlock) GetLocationID() string { return b.LocationID }
func (b *DividerBlock) GetName() string       { return "Divider" }
func (b *DividerBlock) GetDescription() string {
	return "Simple divider to separate content."
}
func (b *DividerBlock) GetOrder() int  { return b.Order }
func (b *DividerBlock) GetPoints() int { return b.Points }
func (b *DividerBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-minus"><path d="M5 12h14"/></svg>`
}
func (b *DividerBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *DividerBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *DividerBlock) UpdateBlockData(input map[string][]string) error {
	if title, exists := input["title"]; exists && len(title) > 0 {
		b.Title = title[0]
	}
	return nil
}

// Validation and Points Calculation

func (b *DividerBlock) RequiresValidation() bool {
	return false
}

func (b *DividerBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required for DividerBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}
