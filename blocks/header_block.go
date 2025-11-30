package blocks

import (
	"encoding/json"
	"errors"
)

type HeaderBlock struct {
	BaseBlock
	Icon      string `json:"icon"`
	TitleText string `json:"title_text"`
	TitleSize string `json:"title_size"` // small, medium, large
}

// Basic Attributes Getters

func (b *HeaderBlock) GetID() string         { return b.ID }
func (b *HeaderBlock) GetType() string       { return "header" }
func (b *HeaderBlock) GetLocationID() string { return b.LocationID }
func (b *HeaderBlock) GetName() string       { return "Header" }
func (b *HeaderBlock) GetDescription() string {
	return "Display a customizable header with logo and title at the top of the page."
}
func (b *HeaderBlock) GetOrder() int  { return b.Order }
func (b *HeaderBlock) GetPoints() int { return b.Points }
func (b *HeaderBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-header"><path d="M6 12h12"/><path d="M6 20V4"/><path d="M18 20V4"/></svg>`
}
func (b *HeaderBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *HeaderBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *HeaderBlock) UpdateBlockData(input map[string][]string) error {
	icon, iconExists := input["icon"]
	titleText, titleExists := input["title_text"]
	if len(icon[0]) == 0 && len(titleText[0]) == 0 {
		return errors.New("title text or icon must be provided")
	}
	if iconExists && len(icon) > 0 {
		b.Icon = icon[0]
	} else {
		b.Icon = ""
	}
	if titleExists && len(titleText) > 0 {
		b.TitleText = titleText[0]
	}
	titleSize, exists := input["title_size"]
	if exists && len(titleSize) > 0 {
		b.TitleSize = titleSize[0]
	}
	return nil
}

// Validation and Points Calculation

func (b *HeaderBlock) RequiresValidation() bool {
	return false
}

func (b *HeaderBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required for HeaderBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}

func (b *HeaderBlock) GetTitleSizes() map[string]string {
	return map[string]string{
		"small":  "Small",
		"medium": "Medium",
		"large":  "Large",
	}
}
