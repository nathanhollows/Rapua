package blocks

import (
	"encoding/json"
)

type MarkdownBlock struct {
	BaseBlock
	Content string `json:"content"`
}

// Basic Attributes Getters

func (b *MarkdownBlock) GetID() string         { return b.ID }
func (b *MarkdownBlock) GetType() string       { return "markdown" }
func (b *MarkdownBlock) GetLocationID() string { return b.LocationID }
func (b *MarkdownBlock) GetName() string       { return "Text" }
func (b *MarkdownBlock) GetDescription() string {
	return "Text (Supports Markdown)"
}
func (b *MarkdownBlock) GetOrder() int  { return b.Order }
func (b *MarkdownBlock) GetPoints() int { return b.Points }
func (b *MarkdownBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-case-sensitive"><path d="m3 15 4-8 4 8"/><path d="M4 13h6"/><circle cx="18" cy="12" r="3"/><path d="M21 9v6"/></svg>`
}
func (b *MarkdownBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *MarkdownBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *MarkdownBlock) UpdateBlockData(input map[string][]string) error {
	if content, exists := input["content"]; exists && len(content) > 0 {
		b.Content = content[0]
	}
	return nil
}

// Validation and Points Calculation

func (b *MarkdownBlock) RequiresValidation() bool {
	return false
}

func (b *MarkdownBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required for MarkdownBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}
