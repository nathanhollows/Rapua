package blocks

import (
	"encoding/json"
	"errors"
	"strconv"
)

type FreeTextBlock struct {
	BaseBlock
	Prompt      string `json:"prompt"`
	Placeholder string `json:"placeholder"`
}

type freeTextPlayerData struct {
	Response string `json:"response"`
}

// Basic Attributes Getters

func (b *FreeTextBlock) GetID() string      { return b.ID }
func (b *FreeTextBlock) GetType() string    { return "free_text" }
func (b *FreeTextBlock) GetOwnerID() string { return b.OwnerID }
func (b *FreeTextBlock) GetName() string    { return "Free Text" }
func (b *FreeTextBlock) GetDescription() string {
	return "Players write a free text response"
}
func (b *FreeTextBlock) GetOrder() int  { return b.Order }
func (b *FreeTextBlock) GetPoints() int { return b.Points }
func (b *FreeTextBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-pen-line-icon lucide-pen-line"><path d="M13 21h8"/><path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/></svg>`
}

func (b *FreeTextBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *FreeTextBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *FreeTextBlock) UpdateBlockData(input map[string][]string) error {
	if input["points"] != nil {
		points, err := strconv.Atoi(input["points"][0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	}
	if prompt, exists := input["prompt"]; exists && len(prompt) > 0 {
		b.Prompt = prompt[0]
	}
	if placeholder, exists := input["placeholder"]; exists && len(placeholder) > 0 {
		b.Placeholder = placeholder[0]
	}
	return nil
}

// ToYAML returns the block's data for YAML export.
func (b *FreeTextBlock) ToYAML() map[string]any {
	m := map[string]any{
		"prompt": b.Prompt,
	}
	if b.Placeholder != "" {
		m["placeholder"] = b.Placeholder
	}
	return m
}

// Validation and Points Calculation

func (b *FreeTextBlock) RequiresValidation() bool { return true }

func (b *FreeTextBlock) ValidatePlayerInput(
	state PlayerState,
	input map[string][]string,
) (PlayerState, error) {
	var playerData freeTextPlayerData
	if state.GetPlayerData() != nil {
		if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
			return state, errors.New("failed to parse player data")
		}
	}

	if response, exists := input["response"]; exists && len(response) > 0 {
		playerData.Response = response[0]
	}

	newPlayerData, err := json.Marshal(playerData)
	if err != nil {
		return state, errors.New("error saving player data")
	}
	state.SetPlayerData(newPlayerData)

	if playerData.Response != "" {
		state.SetComplete(true)
		state.SetPointsAwarded(b.Points)
	} else {
		state.SetComplete(false)
		state.SetPointsAwarded(0)
	}

	return state, nil
}

// GetResponse extracts the player's text response from state.
func (b *FreeTextBlock) GetResponse(state PlayerState) string {
	if state == nil || state.GetPlayerData() == nil {
		return ""
	}
	var data freeTextPlayerData
	if err := json.Unmarshal(state.GetPlayerData(), &data); err != nil {
		return ""
	}
	return data.Response
}
