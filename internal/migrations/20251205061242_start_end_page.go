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

// Block data structures for migration (self-contained).
//
//nolint:revive // Migration-specific naming convention
type m20251205061242_HeaderBlockData struct {
	Icon      string `json:"icon"`
	TitleText string `json:"title_text"`
	TitleSize string `json:"title_size"`
}

//nolint:revive // Migration-specific naming convention
type m20251205061242_DividerBlockData struct {
	Title string `json:"title"`
}

//nolint:revive // Migration-specific naming convention
type m20251205061242_MarkdownBlockData struct {
	Content string `json:"content"`
}

//nolint:revive // Migration-specific naming convention
type m20251205061242_TeamNameChangerBlockData struct {
	BlockText     string `json:"block_text"`
	AllowChanging bool   `json:"allow_changing"`
}

//nolint:revive // Migration-specific naming convention
type m20251205061242_StartGameButtonBlockData struct {
	ScheduledButtonText string `json:"scheduled_button_text"`
	ActiveButtonText    string `json:"active_button_text"`
	ButtonStyle         string `json:"button_style"`
}

//nolint:revive // Migration-specific naming convention
type m20251205061242_GameStatusAlertBlockData struct {
	ClosedMessage    string `json:"closed_message"`
	ScheduledMessage string `json:"scheduled_message"`
	ShowCountdown    bool   `json:"show_countdown"`
}

//nolint:revive // Migration-specific naming convention
const m20251205061242_LobbyInstructions = `` +
	`- Navigate to each location using the clues, maps, or directions provided.
- When you arrive, check in by scanning the QR code or following the link.
- Complete the activity at each stop.
- Continue moving through all locations and completing their activities until you reach the final checkpoint.
- Have fun exploring!`

//nolint:revive // Migration-specific naming convention
const m20251205061242_FinishCongratulations = `` +
	`You've wrapped up the entire route. Thanks for being part of the adventure.`

func init() { //nolint:gocognit,gochecknoinits // Migration init is required
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// PART 1: Add header blocks to all existing locations
		var locations []models.Location
		err := db.NewSelect().
			Model(&locations).
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch locations: %w", err)
		}

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

			// Create header block for location
			headerData, _ := json.Marshal(m20251205061242_HeaderBlockData{
				Icon:      "map-pin-check-inside",
				TitleText: location.Name,
				TitleSize: "large",
			})

			newBlock := models.Block{
				ID:                 uuid.New().String(),
				OwnerID:            location.ID,
				Type:               "header",
				Context:            blocks.ContextLocationContent,
				Data:               headerData,
				Ordering:           0,
				Points:             0,
				ValidationRequired: false,
			}

			_, err = db.NewInsert().Model(&newBlock).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to insert header block for location %s: %w", location.ID, err)
			}
		}

		// PART 2: Add lobby and finish blocks to all existing instances
		var instances []models.Instance
		err = db.NewSelect().
			Model(&instances).
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch instances: %w", err)
		}

		for _, instance := range instances {
			// Create lobby blocks
			lobbyBlocks := m20251205061242_createLobbyBlocks(instance.ID, instance.Name)
			if len(lobbyBlocks) > 0 {
				_, err = db.NewInsert().Model(&lobbyBlocks).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to insert lobby blocks for instance %s: %w", instance.ID, err)
				}
			}

			// Create finish blocks
			finishBlocks := m20251205061242_createFinishBlocks(instance.ID)
			if len(finishBlocks) > 0 {
				_, err = db.NewInsert().Model(&finishBlocks).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to insert finish blocks for instance %s: %w", instance.ID, err)
				}
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// ROLLBACK PART 1: Delete location header blocks
		_, err := db.NewDelete().
			Model((*models.Block)(nil)).
			Where("type = ?", "header").
			Where("context = ?", blocks.ContextLocationContent).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete location header blocks: %w", err)
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

		// ROLLBACK PART 2: Delete all lobby and finish blocks
		_, err = db.NewDelete().
			Model((*models.Block)(nil)).
			Where("context IN (?, ?)", blocks.ContextStart, blocks.ContextFinish).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete lobby/finish blocks: %w", err)
		}

		return nil
	})
}

// m20251205061242_createLobbyBlocks creates the default blocks for an instance's start/lobby page.
//
//nolint:revive // Migration-specific naming convention
func m20251205061242_createLobbyBlocks(instanceID, instanceName string) []models.Block {
	result := make([]models.Block, 7) //nolint:mnd // 7 blocks for lobby page

	// 1. Header
	headerData, _ := json.Marshal(m20251205061242_HeaderBlockData{
		Icon:      "map-pin-check-inside",
		TitleText: instanceName,
		TitleSize: "large",
	})
	result[0] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "header",
		Context:            blocks.ContextStart,
		Data:               headerData,
		Ordering:           0,
		Points:             0,
		ValidationRequired: false,
	}

	// 2. Game status alert
	gameStatusData, _ := json.Marshal(m20251205061242_GameStatusAlertBlockData{
		ClosedMessage:    "This game is not yet open.",
		ScheduledMessage: "This game will start soon.",
		ShowCountdown:    true,
	})
	result[1] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "game_status_alert",
		Context:            blocks.ContextStart,
		Data:               gameStatusData,
		Ordering:           1,
		Points:             0,
		ValidationRequired: false,
	}

	// 3. Divider - Instructions
	divider1Data, _ := json.Marshal(m20251205061242_DividerBlockData{Title: "How to play"})
	result[2] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "divider",
		Context:            blocks.ContextStart,
		Data:               divider1Data,
		Ordering:           2, //nolint:mnd // Sequential ordering
		Points:             0,
		ValidationRequired: false,
	}

	// 4. Markdown - Instructions
	markdownData, _ := json.Marshal(m20251205061242_MarkdownBlockData{Content: m20251205061242_LobbyInstructions})
	result[3] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "markdown",
		Context:            blocks.ContextStart,
		Data:               markdownData,
		Ordering:           3, //nolint:mnd // Sequential ordering
		Points:             0,
		ValidationRequired: false,
	}

	// 5. Divider - Team Info
	divider2Data, _ := json.Marshal(m20251205061242_DividerBlockData{Title: "Team Info"})
	result[4] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "divider",
		Context:            blocks.ContextStart,
		Data:               divider2Data,
		Ordering:           4, //nolint:mnd // Sequential ordering
		Points:             0,
		ValidationRequired: false,
	}

	// 6. Team name changer
	teamNameData, _ := json.Marshal(m20251205061242_TeamNameChangerBlockData{
		BlockText:     "Set your team name",
		AllowChanging: true,
	})
	result[5] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "team_name",
		Context:            blocks.ContextStart,
		Data:               teamNameData,
		Ordering:           5, //nolint:mnd // Sequential ordering
		Points:             0,
		ValidationRequired: true, // TeamNameChangerBlock requires validation
	}

	// 7. Start game button
	startData, _ := json.Marshal(m20251205061242_StartGameButtonBlockData{
		ScheduledButtonText: "Game starts soon...",
		ActiveButtonText:    "Start Game",
		ButtonStyle:         "primary",
	})
	result[6] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "start_game_button",
		Context:            blocks.ContextStart,
		Data:               startData,
		Ordering:           6, //nolint:mnd // Sequential ordering
		Points:             0,
		ValidationRequired: false,
	}

	return result
}

// m20251205061242_createFinishBlocks creates the default blocks for an instance's finish page.
//
//nolint:revive // Migration-specific naming convention
func m20251205061242_createFinishBlocks(instanceID string) []models.Block {
	result := make([]models.Block, 2) //nolint:mnd // 2 blocks for finish page

	// 1. Header
	headerData, _ := json.Marshal(m20251205061242_HeaderBlockData{
		Icon:      "party-popper",
		TitleText: "Congratulations!",
		TitleSize: "large",
	})
	result[0] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "header",
		Context:            blocks.ContextFinish,
		Data:               headerData,
		Ordering:           0,
		Points:             0,
		ValidationRequired: false,
	}

	// 2. Markdown - Congratulations
	markdownData, _ := json.Marshal(m20251205061242_MarkdownBlockData{Content: m20251205061242_FinishCongratulations})
	result[1] = models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "markdown",
		Context:            blocks.ContextFinish,
		Data:               markdownData,
		Ordering:           1,
		Points:             0,
		ValidationRequired: false,
	}

	return result
}
