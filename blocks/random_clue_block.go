package blocks

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
)

type RandomClueBlock struct {
	BaseBlock
	Clues []string `json:"clues"`
}

// Basic Attributes Getters

func (b *RandomClueBlock) GetID() string         { return b.ID }
func (b *RandomClueBlock) GetType() string       { return "random_clue" }
func (b *RandomClueBlock) GetLocationID() string { return b.LocationID }
func (b *RandomClueBlock) GetName() string       { return "Random Clue" }
func (b *RandomClueBlock) GetDescription() string {
	return "Display a random clue deterministically selected for each team."
}
func (b *RandomClueBlock) GetOrder() int  { return b.Order }
func (b *RandomClueBlock) GetPoints() int { return b.Points }
func (b *RandomClueBlock) GetIconSVG() string {
	return `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-search-question-icon lucide-search-question" xmlns="http://www.w3.org/2000/svg"><defs id="defs1" /> <path d="m21 21-4.34-4.34" id="path1" /> <circle cx="11" cy="11" r="8" id="circle1" /> <path d="m 8.989691,8.3078121 c 0.2,-0.4 0.5,-0.8 0.9,-1 a 2.1,2.1 0 0 1 2.6,0.4 c 0.3,0.4 0.5,0.8 0.5,1.3 0,1.2999999 -2,1.9999999 -2,1.9999999" id="path3" /> <path d="m 10.989691,15.027812 v 0.01" id="path4" /></svg>`
}
func (b *RandomClueBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *RandomClueBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *RandomClueBlock) UpdateBlockData(input map[string][]string) error {
	// Handle clues array
	if clues, exists := input["clues"]; exists {
		var newClues []string
		for _, content := range clues {
			if content != "" { // Only add non-empty clues
				newClues = append(newClues, content)
			}
		}
		b.Clues = newClues
	}
	return nil
}

// GetClue returns a deterministic random clue based on the team code
func (b *RandomClueBlock) GetClue(teamCode string) string {
	if len(b.Clues) == 0 {
		return "No clues available"
	}

	// Create deterministic hash from team code
	hash := sha256.Sum256([]byte(teamCode + b.ID))
	seed := binary.BigEndian.Uint64(hash[:8])

	// Select clue based on hash
	index := seed % uint64(len(b.Clues))
	return b.Clues[index]
}

// Validation and Points Calculation

func (b *RandomClueBlock) RequiresValidation() bool { return false }

func (b *RandomClueBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	// No validation needed - this is a display-only block
	state.SetComplete(true)
	return state, nil
}
