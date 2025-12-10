package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

// HeaderBlockData represents the JSON structure for header blocks
type m20251205061242_HeaderBlockData struct {
	Icon      string `json:"icon"`
	TitleText string `json:"title_text"`
	TitleSize string `json:"title_size"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Get all locations
		var locations []models.Location
		err := db.NewSelect().
			Model(&locations).
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch locations: %w", err)
		}

		// Process each location
		for _, location := range locations {
			// Bump all location_content blocks ordering by 1 (if any exist)
			_, err = db.NewUpdate().
				Model((*models.Block)(nil)).
				Set("ordering = ordering + 1").
				Where("owner_id = ?", location.ID).
				Where("context = ?", blocks.ContextLocationContent).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to update block ordering for location %s: %w", location.ID, err)
			}

			// Create header block data
			blockID := uuid.New().String()
			headerData := m20251205061242_HeaderBlockData{
				Icon:      "map-pin-check-inside",
				TitleText: location.Name,
				TitleSize: "large",
			}

			jsonData, jsonErr := json.Marshal(headerData)
			if jsonErr != nil {
				return fmt.Errorf("failed to marshal header block data for location %s: %w", location.ID, jsonErr)
			}

			// Insert new header block at position 0
			newBlock := models.Block{
				ID:                 blockID,
				OwnerID:            location.ID,
				Type:               "header",
				Context:            blocks.ContextLocationContent,
				Data:               jsonData,
				Ordering:           0,
				Points:             0,
				ValidationRequired: false,
			}

			_, err = db.NewInsert().
				Model(&newBlock).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to insert header block for location %s: %w", location.ID, err)
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Delete all header blocks with context = location_content
		_, err := db.NewDelete().
			Model((*models.Block)(nil)).
			Where("type = ?", "header").
			Where("context = ?", blocks.ContextLocationContent).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete header blocks: %w", err)
		}

		// Decrement ordering for all remaining location_content blocks
		_, err = db.NewUpdate().
			Model((*models.Block)(nil)).
			Set("ordering = ordering - 1").
			Where("context = ?", blocks.ContextLocationContent).
			Where("ordering > 0").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to restore block ordering: %w", err)
		}

		return nil
	})
}
