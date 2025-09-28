package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Migration models for this specific migration
type m20250927115843_Clue struct {
	bun.BaseModel `bun:"table:clues"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID         string `bun:"id,pk,type:varchar(36)"`
	InstanceID string `bun:"instance_id,notnull"`
	LocationID string `bun:"location_id,notnull"`
	Content    string `bun:"content,type:text"`
}

type m20250927115843_Location struct {
	bun.BaseModel `bun:"table:locations"`

	ID         string `bun:"id,pk,notnull"`
	InstanceID string `bun:"instance_id,notnull"`
}

// RandomClueBlockData represents the JSON structure for random_clue blocks
type m20250927115843_RandomClueBlockData struct {
	Clues []string `json:"clues"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {

		// Get all clues grouped by location
		var clues []m20250927115843_Clue
		err := db.NewSelect().
			Model(&clues).
			Order("location_id", "created_at").
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch clues: %w", err)
		}

		// Group clues by location
		cluesByLocation := make(map[string][]string)
		for _, clue := range clues {
			content := clue.Content
			// Add "> " prefix if not already present
			if !strings.HasPrefix(content, "> ") {
				content = "> " + content
			}
			cluesByLocation[clue.LocationID] = append(cluesByLocation[clue.LocationID], content)
		}

		// Create random_clue blocks for each location that has clues
		for locationID, locationClues := range cluesByLocation {
			if len(locationClues) == 0 {
				continue
			}

			// Create the JSON data for the random_clue block
			blockData := m20250927115843_RandomClueBlockData{
				Clues: locationClues,
			}

			jsonData, err := json.Marshal(blockData)
			if err != nil {
				return fmt.Errorf("failed to marshal block data for location %s: %w", locationID, err)
			}

			// Create the block
			block := m20250926120511_Block{
				ID:                 uuid.New().String(),
				LocationID:         locationID,
				Type:               "random_clue",
				Context:            "location_nav_content", // For nav display
				Data:               jsonData,
				Ordering:           0, // Default ordering
				Points:             0, // Default points
				ValidationRequired: false,
			}

			_, err = db.NewInsert().Model(&block).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to insert random_clue block for location %s: %w", locationID, err)
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {

		// Get all random_clue blocks with nav context (these were created from clues)
		var blocks []m20250926120511_Block
		err := db.NewSelect().
			Model(&blocks).
			Where("type = ? AND context = ?", "random_clue", "location_nav_content").
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch random_clue blocks: %w", err)
		}

		cluesCreated := 0
		for _, block := range blocks {
			// Get the InstanceID from the location
			var location m20250927115843_Location
			err := db.NewSelect().
				Model(&location).
				Where("id = ?", block.LocationID).
				Scan(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch location for block %s: %w", block.ID, err)
			}

			// Parse the block data to extract clues
			var blockData m20250927115843_RandomClueBlockData
			err = json.Unmarshal(block.Data, &blockData)
			if err != nil {
				return fmt.Errorf("failed to unmarshal block data for block %s: %w", block.ID, err)
			}

			// Create individual clue records for each clue in the block
			for _, clueContent := range blockData.Clues {
				// Remove the "> " prefix when converting back to clues
				content := clueContent
				content = strings.TrimPrefix(content, "> ")

				clue := m20250927115843_Clue{
					ID:         uuid.New().String(),
					InstanceID: location.InstanceID,
					LocationID: block.LocationID,
					Content:    content,
				}

				_, err = db.NewInsert().Model(&clue).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to insert clue for block %s: %w", block.ID, err)
				}
				cluesCreated++
			}
		}

		// Now remove the random_clue blocks with nav context
		_, err = db.NewDelete().
			Model((*m20250926120511_Block)(nil)).
			Where("type = ? AND context = ?", "random_clue", "location_nav_content").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete random_clue blocks: %w", err)
		}

		return nil
	})
}
