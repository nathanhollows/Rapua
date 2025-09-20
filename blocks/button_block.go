package blocks

import (
	"encoding/json"
)

type ButtonBlock struct {
	BaseBlock
	Link    string `json:"link"`
	Text    string `json:"text"`
	Variant string `json:"variant"`
}

// Basic Attributes Getters

func (b *ButtonBlock) GetID() string         { return b.ID }
func (b *ButtonBlock) GetType() string       { return "button" }
func (b *ButtonBlock) GetLocationID() string { return b.LocationID }
func (b *ButtonBlock) GetName() string       { return "Button" }
func (b *ButtonBlock) GetDescription() string {
	return "Show a clickable button"
}
func (b *ButtonBlock) GetOrder() int  { return b.Order }
func (b *ButtonBlock) GetPoints() int { return b.Points }
func (b *ButtonBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-link-icon lucide-link"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>`
}
func (b *ButtonBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *ButtonBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *ButtonBlock) UpdateBlockData(input map[string][]string) error {
	if variant, exists := input["variant"]; exists && len(variant) > 0 {
		b.Variant = variant[0]
	}
	if link, exists := input["link"]; exists && len(link) > 0 {
		b.Link = link[0]
	}
	if text, exists := input["text"]; exists && len(text) > 0 {
		b.Text = text[0]
	}
	return nil
}

// Validation and Points Calculation

func (b *ButtonBlock) RequiresValidation() bool {
	return false
}

func (b *ButtonBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	// No validation required for ButtonBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}

func (b *ButtonBlock) GetVariants() map[string]string {
	return map[string]string{
		"":          "Default",
		"primary":   "Primary",
		"secondary": "Secondary",
		"accent":    "Accent",
		"info":      "Info",
		"success":   "Success",
		"warning":   "Warning",
		"error":     "Error",
	}
}
