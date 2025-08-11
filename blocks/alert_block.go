package blocks

import (
	"encoding/json"
)

type AlertBlock struct {
	BaseBlock
	Content string `json:"content"`
	Variant string `json:"variant"`
}

// Basic Attributes Getters

func (b *AlertBlock) GetID() string         { return b.ID }
func (b *AlertBlock) GetType() string       { return "alert" }
func (b *AlertBlock) GetLocationID() string { return b.LocationID }
func (b *AlertBlock) GetName() string       { return "Alert" }
func (b *AlertBlock) GetDescription() string {
	return "Display a message to the player."
}
func (b *AlertBlock) GetOrder() int  { return b.Order }
func (b *AlertBlock) GetPoints() int { return b.Points }
func (b *AlertBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-info"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`
}
func (b *AlertBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *AlertBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *AlertBlock) UpdateBlockData(input map[string][]string) error {
	if variant, exists := input["variant"]; exists && len(variant) > 0 {
		b.Variant = variant[0]
	}
	if content, exists := input["content"]; exists && len(content) > 0 {
		b.Content = content[0]
	}
	return nil
}

// Validation and Points Calculation

func (b *AlertBlock) RequiresValidation() bool {
	return false
}

func (b *AlertBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	// No validation required for AlertBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}

func (b *AlertBlock) GetVariants() map[string]string {
	return map[string]string{
		"":        "Default",
		"info":    "Info",
		"success": "Success",
		"warning": "Warning",
		"error":   "Error",
	}
}
