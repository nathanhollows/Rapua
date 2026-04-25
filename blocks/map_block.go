package blocks

import (
	"encoding/json"
	"errors"
	"strconv"
)

const mapBlockType = "map"

type MapBlock struct {
	BaseBlock
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Zoom       float64 `json:"zoom"`
	Caption    string  `json:"caption"`
	HideMarker bool    `json:"hide_marker"`
}

// Basic Attributes Getters

func (b *MapBlock) GetID() string      { return b.ID }
func (b *MapBlock) GetType() string    { return mapBlockType }
func (b *MapBlock) GetOwnerID() string { return b.OwnerID }
func (b *MapBlock) GetName() string    { return "Map" }
func (b *MapBlock) GetDescription() string {
	return "Display an interactive map centred on a specific location."
}
func (b *MapBlock) GetOrder() int  { return b.Order }
func (b *MapBlock) GetPoints() int { return b.Points }
func (b *MapBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-map-pinned-icon lucide-map-pinned"><path d="M18 8c0 3.613-3.869 7.429-5.393 8.795a1 1 0 0 1-1.214 0C9.87 15.429 6 11.613 6 8a6 6 0 0 1 12 0"/><circle cx="12" cy="8" r="2"/><path d="M8.714 14h-3.71a1 1 0 0 0-.948.683l-2.004 6A1 1 0 0 0 3 22h18a1 1 0 0 0 .948-1.316l-2-6a1 1 0 0 0-.949-.684h-3.712"/></svg>`
}
func (b *MapBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *MapBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *MapBlock) UpdateBlockData(input map[string][]string) error {
	if lat, exists := input["latitude"]; exists && len(lat) > 0 && lat[0] != "" {
		v, err := strconv.ParseFloat(lat[0], 64)
		if err != nil {
			return errors.New("latitude must be a number")
		}
		b.Latitude = v
	}
	if lng, exists := input["longitude"]; exists && len(lng) > 0 && lng[0] != "" {
		v, err := strconv.ParseFloat(lng[0], 64)
		if err != nil {
			return errors.New("longitude must be a number")
		}
		b.Longitude = v
	}
	if zoom, exists := input["zoom"]; exists && len(zoom) > 0 && zoom[0] != "" {
		v, err := strconv.ParseFloat(zoom[0], 64)
		if err != nil {
			return errors.New("zoom must be a number")
		}
		b.Zoom = v
	}
	if caption, exists := input["caption"]; exists && len(caption) > 0 {
		b.Caption = caption[0]
	}
	// Checkbox: present = checked = show marker; absent = unchecked = hide marker
	_, showMarker := input["show_marker"]
	b.HideMarker = !showMarker
	return nil
}

// ToYAML returns the block's data for YAML export.
func (b *MapBlock) ToYAML() map[string]any {
	m := map[string]any{
		"latitude":  b.Latitude,
		"longitude": b.Longitude,
	}
	if b.Zoom != 0 {
		m["zoom"] = b.Zoom
	}
	if b.Caption != "" {
		m["caption"] = b.Caption
	}
	if b.HideMarker {
		m["hide_marker"] = true
	}
	return m
}

// Validation and Points Calculation

func (b *MapBlock) RequiresValidation() bool {
	return false
}

func (b *MapBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	state.SetComplete(true)
	return state, nil
}
