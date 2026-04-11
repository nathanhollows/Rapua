package blocks

import (
	"encoding/json"
)

type ToggleTextBlock struct {
	BaseBlock
	Title   string `json:"title"`
	Content string `json:"content"`
	Small   bool   `json:"small"`
}

// Basic Attributes Getters

func (b *ToggleTextBlock) GetID() string      { return b.ID }
func (b *ToggleTextBlock) GetType() string    { return "toggle_text" }
func (b *ToggleTextBlock) GetOwnerID() string { return b.OwnerID }
func (b *ToggleTextBlock) GetName() string    { return "Toggle Text" }
func (b *ToggleTextBlock) GetDescription() string {
	return "Collapsible content with a title, useful for hints, references, or optional detail."
}
func (b *ToggleTextBlock) GetOrder() int  { return b.Order }
func (b *ToggleTextBlock) GetPoints() int { return b.Points }
func (b *ToggleTextBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevrons-up-down"><path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/></svg>`
}
func (b *ToggleTextBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *ToggleTextBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *ToggleTextBlock) UpdateBlockData(input map[string][]string) error {
	if title, exists := input["title"]; exists && len(title) > 0 {
		b.Title = title[0]
	}
	if content, exists := input["content"]; exists && len(content) > 0 {
		b.Content = content[0]
	}
	_, b.Small = input["small"]
	return nil
}

// ToYAML returns the block's data for YAML export.
func (b *ToggleTextBlock) ToYAML() map[string]any {
	m := map[string]any{
		"title":   b.Title,
		"content": b.Content,
	}
	if b.Small {
		m["small"] = true
	}
	return m
}

// Validation and Points Calculation

func (b *ToggleTextBlock) RequiresValidation() bool {
	return false
}

func (b *ToggleTextBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	state.SetComplete(true)
	return state, nil
}
