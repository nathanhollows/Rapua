package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// MediaType represents the type of media being uploaded.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

// Upload represents a file that has been uploaded to the system.
type Upload struct {
	bun.BaseModel `bun:"table:uploads,alias:u"`

	ID          string    `bun:"id,pk,notnull"`
	OriginalURL string    `bun:"original_url,notnull"` // Original file link
	Timestamp   time.Time `bun:"timestamp"`
	LocationID  string    `bun:"location_id,nullzero"`
	InstanceID  string    `bun:"instance_id,nullzero"`
	TeamCode    string    `bun:"team_code,nullzero"`
	BlockID     string    `bun:"block_id,nullzero"`
	Storage     string    `bun:"storage,notnull"`
	DeleteData  string    `bun:"delete_data"`
	Type        MediaType `bun:"type"`
	sizes       string    `bun:"sizes"` // Stores JSON string of different filesizes
}

// ImageSize represents an image variant with a specific breakpoint.
type ImageSize struct {
	Breakpoint int    `json:"breakpoint"` // px width for media queries
	URL        string `json:"url"`        // URL to the image
}

// ** Helper Functions **

// GetSizes retrieves image sizes as a structured list.
func (u *Upload) GetSizes() ([]ImageSize, error) {
	if u.sizes == "" {
		return []ImageSize{}, nil
	}

	var sizes []ImageSize
	err := json.Unmarshal([]byte(u.sizes), &sizes)
	if err != nil {
		return []ImageSize{}, err
	}
	return sizes, nil
}

// AddSize appends a new image size to the existing sizes.
func (u *Upload) AddSize(breakpoint int, url string) error {
	sizes, err := u.GetSizes()
	if err != nil {
		return errors.New("failed to get sizes")
	}

	if sizes == nil {
		sizes = []ImageSize{}
	}

	sizes = append(sizes, ImageSize{Breakpoint: breakpoint, URL: url})

	bytes, err := json.Marshal(sizes)
	if err != nil {
		return err
	}
	u.sizes = string(bytes)
	return nil
}
