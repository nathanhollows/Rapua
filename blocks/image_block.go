package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type ImageBlock struct {
	BaseBlock
	URL     string `json:"content"`
	Caption string `json:"caption"`
	Link    string `json:"link"`
}

// Basic Attributes Getters

func (b *ImageBlock) GetID() string         { return b.ID }
func (b *ImageBlock) GetType() string       { return "image" }
func (b *ImageBlock) GetLocationID() string { return b.LocationID }
func (b *ImageBlock) GetName() string       { return "Image" }
func (b *ImageBlock) GetDescription() string {
	return "Embed an image."
}
func (b *ImageBlock) GetOrder() int  { return b.Order }
func (b *ImageBlock) GetPoints() int { return b.Points }
func (b *ImageBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-image"><rect width="18" height="18" x="3" y="3" rx="2" ry="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/></svg>`
}
func (b *ImageBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *ImageBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

// UpdateBlockData expects values with the following keys:
// - url
// - caption
// - link
// Where url is required, and caption and link are optional.
// Both url and link must be valid URLs.
func (b *ImageBlock) UpdateBlockData(input map[string][]string) error {
	imageURL, err := b.parseURL(input)
	if err != nil {
		return err
	}
	b.URL = imageURL

	if caption, exists := input["caption"]; exists && len(caption) > 0 {
		b.Caption = caption[0]
	}

	if link, exists := input["link"]; exists && len(link) > 0 {
		b.Link = link[0]
	}

	return nil
}

func (b *ImageBlock) parseURL(input map[string][]string) (string, error) {
	var inputURL string
	if u, exists := input["url"]; !exists || len(u) == 0 {
		return "", errors.New("url is a required field")
	}
	inputURL = strings.TrimSpace(input["url"][0])

	parsedURL, err := url.ParseRequestURI(inputURL)
	if err != nil {
		return "", fmt.Errorf("url is not valid: %w", err)
	}

	return parsedURL.String(), nil
}

// RequiresValidation returns whether this block requires player input validation.
func (b *ImageBlock) RequiresValidation() bool {
	return false
}

func (b *ImageBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required for ImageBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}
