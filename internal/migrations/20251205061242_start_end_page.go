package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/uptrace/bun"
)

type m20251205061242_Location struct {
	bun.BaseModel `bun:"table:locations"`
	ID            string `bun:"id,pk,notnull"`
	Name          string `bun:"name"`
}

type m20251205061242_Instance struct {
	bun.BaseModel `bun:"table:instances"`
	ID            string `bun:"id,pk,notnull"`
	Name          string `bun:"name"`
}

type m20251205061242_Block struct {
	bun.BaseModel      `bun:"table:blocks"`
	ID                 string              `bun:"id,pk,notnull"`
	OwnerID            string              `bun:"owner_id,notnull"`
	Type               string              `bun:"type"`
	Context            blocks.BlockContext `bun:"context"`
	Data               json.RawMessage     `bun:"data"`
	Ordering           int                 `bun:"ordering"`
	Points             int                 `bun:"points"`
	ValidationRequired bool                `bun:"validation_required"`
}

type m20251205061242_HeaderBlockData struct {
	Icon      string `json:"icon"`
	TitleText string `json:"title_text"`
	TitleSize string `json:"title_size"`
}

type m20251205061242_DividerBlockData struct {
	Title string `json:"title"`
}

type m20251205061242_MarkdownBlockData struct {
	Content string `json:"content"`
}

type m20251205061242_TeamNameChangerBlockData struct {
	BlockText     string `json:"block_text"`
	AllowChanging bool   `json:"allow_changing"`
}

type m20251205061242_StartGameButtonBlockData struct {
	ScheduledButtonText string `json:"scheduled_button_text"`
	ActiveButtonText    string `json:"active_button_text"`
	ButtonStyle         string `json:"button_style"`
}

type m20251205061242_GameStatusAlertBlockData struct {
	ClosedMessage    string `json:"closed_message"`
	ScheduledMessage string `json:"scheduled_message"`
	ShowCountdown    bool   `json:"show_countdown"`
}

const m20251205061242_StartInstructions = `` +
	`- Navigate to each location using the clues, maps, or directions provided.
- When you arrive, check in by scanning the QR code or following the link.
- Complete the activity at each stop.
- Continue moving through all locations and completing their activities until you reach the final checkpoint.
- Have fun exploring!`

const m20251205061242_FinishCongratulations = `` +
	`You've wrapped up the entire route. Thanks for being part of the adventure.`

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		var locations []m20251205061242_Location
		err := db.NewSelect().
			Model(&locations).
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch locations: %w", err)
		}

		for _, location := range locations {
			_, err = db.NewUpdate().
				Model((*m20251205061242_Block)(nil)).
				Set("ordering = ordering + 1").
				Where("owner_id = ?", location.ID).
				Where("context = ?", "location_content").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to update block ordering for location %s: %w", location.ID, err)
			}

			headerData, _ := json.Marshal(m20251205061242_HeaderBlockData{
				Icon:      "map-pin-check-inside",
				TitleText: location.Name,
				TitleSize: "large",
			})

			newBlock := m20251205061242_Block{
				ID:                 uuid.New().String(),
				OwnerID:            location.ID,
				Type:               "header",
				Context:            blocks.BlockContext("location_content"),
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

		var instances []m20251205061242_Instance
		err = db.NewSelect().
			Model(&instances).
			Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch instances: %w", err)
		}

		for _, instance := range instances {
			startBlocks := m20251205061242_createStartBlocks(instance.ID, instance.Name)
			if len(startBlocks) > 0 {
				_, err = db.NewInsert().Model(&startBlocks).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to insert start blocks for instance %s: %w", instance.ID, err)
				}
			}

			finishBlocks := m20251205061242_createFinishBlocks(instance.ID)
			if len(finishBlocks) > 0 {
				_, err = db.NewInsert().Model(&finishBlocks).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to insert complete blocks for instance %s: %w", instance.ID, err)
				}
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewDelete().
			Model((*m20251205061242_Block)(nil)).
			Where("type = ?", "header").
			Where("context = ?", "location_content").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete location header blocks: %w", err)
		}

		_, err = db.NewUpdate().
			Model((*m20251205061242_Block)(nil)).
			Set("ordering = ordering - 1").
			Where("context = ?", "location_content").
			Where("ordering > 0").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to restore block ordering: %w", err)
		}

		_, err = db.NewDelete().
			Model((*m20251205061242_Block)(nil)).
			Where("context IN (?, ?)", blocks.ContextStart, blocks.ContextFinish).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete start/complete blocks: %w", err)
		}

		return nil
	})
}

func m20251205061242_createStartBlocks(instanceID, instanceName string) []m20251205061242_Block {
	result := make([]m20251205061242_Block, 7)

	headerData, _ := json.Marshal(m20251205061242_HeaderBlockData{
		Icon:      "map-pin-check-inside",
		TitleText: instanceName,
		TitleSize: "large",
	})
	result[0] = m20251205061242_Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "header",
		Context:            blocks.ContextStart,
		Data:               headerData,
		Ordering:           0,
		Points:             0,
		ValidationRequired: false,
	}

	gameStatusData, _ := json.Marshal(m20251205061242_GameStatusAlertBlockData{
		ClosedMessage:    "This game is not yet open.",
		ScheduledMessage: "This game will start soon.",
		ShowCountdown:    true,
	})
	result[1] = m20251205061242_Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "game_status_alert",
		Context:            blocks.ContextStart,
		Data:               gameStatusData,
		Ordering:           1,
		Points:             0,
		ValidationRequired: false,
	}

	divider1Data, _ := json.Marshal(m20251205061242_DividerBlockData{Title: "How to play"})
	result[2] = m20251205061242_Block{
		ID:       uuid.New().String(),
		OwnerID:  instanceID,
		Type:     "divider",
		Context:  blocks.ContextStart,
		Data:     divider1Data,
		Ordering: 2, Points: 0,
		ValidationRequired: false,
	}

	markdownData, _ := json.Marshal(m20251205061242_MarkdownBlockData{Content: m20251205061242_StartInstructions})
	result[3] = m20251205061242_Block{
		ID:       uuid.New().String(),
		OwnerID:  instanceID,
		Type:     "markdown",
		Context:  blocks.ContextStart,
		Data:     markdownData,
		Ordering: 3, Points: 0,
		ValidationRequired: false,
	}

	divider2Data, _ := json.Marshal(m20251205061242_DividerBlockData{Title: "Team Info"})
	result[4] = m20251205061242_Block{
		ID:       uuid.New().String(),
		OwnerID:  instanceID,
		Type:     "divider",
		Context:  blocks.ContextStart,
		Data:     divider2Data,
		Ordering: 4, Points: 0,
		ValidationRequired: false,
	}

	teamNameData, _ := json.Marshal(m20251205061242_TeamNameChangerBlockData{
		BlockText:     "Set your team name",
		AllowChanging: true,
	})
	result[5] = m20251205061242_Block{
		ID:       uuid.New().String(),
		OwnerID:  instanceID,
		Type:     "team_name",
		Context:  blocks.ContextStart,
		Data:     teamNameData,
		Ordering: 5, Points: 0,
		ValidationRequired: true,
	}

	startData, _ := json.Marshal(m20251205061242_StartGameButtonBlockData{
		ScheduledButtonText: "Game starts soon...",
		ActiveButtonText:    "Start Game",
		ButtonStyle:         "primary",
	})
	result[6] = m20251205061242_Block{
		ID:       uuid.New().String(),
		OwnerID:  instanceID,
		Type:     "start_game_button",
		Context:  blocks.ContextStart,
		Data:     startData,
		Ordering: 6, Points: 0,
		ValidationRequired: false,
	}

	return result
}

func m20251205061242_createFinishBlocks(instanceID string) []m20251205061242_Block {
	result := make([]m20251205061242_Block, 2)

	headerData, _ := json.Marshal(m20251205061242_HeaderBlockData{
		Icon:      "party-popper",
		TitleText: "Congratulations!",
		TitleSize: "large",
	})
	result[0] = m20251205061242_Block{
		ID:                 uuid.New().String(),
		OwnerID:            instanceID,
		Type:               "header",
		Context:            blocks.ContextFinish,
		Data:               headerData,
		Ordering:           0,
		Points:             0,
		ValidationRequired: false,
	}

	markdownData, _ := json.Marshal(m20251205061242_MarkdownBlockData{Content: m20251205061242_FinishCongratulations})
	result[1] = m20251205061242_Block{
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
