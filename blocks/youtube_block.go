package blocks

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

type YoutubeBlock struct {
	BaseBlock
	URL string `json:"content"`
}

// Basic Attributes Getters

func (b *YoutubeBlock) GetID() string         { return b.ID }
func (b *YoutubeBlock) GetType() string       { return "youtube" }
func (b *YoutubeBlock) GetLocationID() string { return b.LocationID }
func (b *YoutubeBlock) GetName() string       { return "Youtube" }
func (b *YoutubeBlock) GetDescription() string {
	return "Embed a Youtube video."
}
func (b *YoutubeBlock) GetOrder() int  { return b.Order }
func (b *YoutubeBlock) GetPoints() int { return b.Points }
func (b *YoutubeBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-youtube"><path d="M2.5 17a24.12 24.12 0 0 1 0-10 2 2 0 0 1 1.4-1.4 49.56 49.56 0 0 1 16.2 0A2 2 0 0 1 21.5 7a24.12 24.12 0 0 1 0 10 2 2 0 0 1-1.4 1.4 49.55 49.55 0 0 1-16.2 0A2 2 0 0 1 2.5 17"/><path d="m10 15 5-3-5-3z"/></svg>`
}
func (b *YoutubeBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *YoutubeBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *YoutubeBlock) UpdateBlockData(input map[string][]string) error {
	if u, exists := input["URL"]; exists && len(u) > 0 {
		u[0] = strings.TrimSpace(u[0])
		// Regex: https://stackoverflow.com/a/6904504
		_, err := regexp.MatchString(`(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?)\/|.*[?&]v=)|youtu\.be\/)([^"&?\/\s]{11})`, u[0])
		if err != nil {
			return errors.New("URL is not valid")
		}

		// Confirm URL is valid
		checkURL := "https://www.youtube.com/oembed?format=json&url=" + u[0]
		resp, err := http.Get(checkURL)
		defer resp.Body.Close()
		if err != nil {
			return errors.New("URL is not valid")
		}
		if resp.StatusCode != 200 {
			return errors.New("URL is not valid")
		}

		// Convert shorts URL to watch URL
		u[0] = strings.Replace(u[0], "/shorts/", "/watch?v=", 1)

		b.URL = u[0]
	}
	return nil
}

// Validation and Points Calculation

func (b *YoutubeBlock) RequiresValidation() bool {
	return false
}

func (b *YoutubeBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	// No validation required for YoutubeBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}
