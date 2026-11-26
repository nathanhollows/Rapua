package blocks

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
)

type RandomTextBlock struct {
	BaseBlock
	Texts []string `json:"texts"`
}

// Basic Attributes Getters

func (b *RandomTextBlock) GetID() string      { return b.ID }
func (b *RandomTextBlock) GetType() string    { return "random_text" }
func (b *RandomTextBlock) GetOwnerID() string { return b.OwnerID }
func (b *RandomTextBlock) GetName() string    { return "Random Text" }
func (b *RandomTextBlock) GetDescription() string {
	return "Display one randomly selected text, deterministically chosen per team."
}
func (b *RandomTextBlock) GetOrder() int  { return b.Order }
func (b *RandomTextBlock) GetPoints() int { return b.Points }
func (b *RandomTextBlock) GetIconSVG() string {
	return `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-search-question-icon lucide-search-question" xmlns="http://www.w3.org/2000/svg"><defs id="defs1" /> <path d="m21 21-4.34-4.34" id="path1" /> <circle cx="11" cy="11" r="8" id="circle1" /> <path d="m 8.989691,8.3078121 c 0.2,-0.4 0.5,-0.8 0.9,-1 a 2.1,2.1 0 0 1 2.6,0.4 c 0.3,0.4 0.5,0.8 0.5,1.3 0,1.2999999 -2,1.9999999 -2,1.9999999" id="path3" /> <path d="m 10.989691,15.027812 v 0.01" id="path4" /></svg>`
}
func (b *RandomTextBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *RandomTextBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *RandomTextBlock) UpdateBlockData(input map[string][]string) error {
	// Handle texts array
	if texts, exists := input["texts"]; exists {
		var newTexts []string
		for _, content := range texts {
			if content != "" { // Only add non-empty texts
				newTexts = append(newTexts, content)
			}
		}
		b.Texts = newTexts
	}
	return nil
}

// GetText returns a deterministic random text based on the team code.
func (b *RandomTextBlock) GetText(teamCode string) string {
	if len(b.Texts) == 0 {
		return "No text available"
	}

	// Create deterministic hash from team code
	hash := sha256.Sum256([]byte(teamCode + b.ID))
	seed := binary.BigEndian.Uint64(hash[:8])

	// Select a text based on the hash
	index := seed % uint64(len(b.Texts))
	return b.Texts[index]
}

// ToYAML returns the block's data for YAML export.
func (b *RandomTextBlock) ToYAML() map[string]any {
	return map[string]any{
		"texts": b.Texts,
	}
}

// Validation and Points Calculation

func (b *RandomTextBlock) RequiresValidation() bool { return false }

func (b *RandomTextBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation needed - this is a display-only block
	state.SetComplete(true)
	return state, nil
}
