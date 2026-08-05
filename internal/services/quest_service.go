package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/models"
)

type QuestService struct {
	instanceRepo         repositories.QuestRepository
	instanceSettingsRepo repositories.QuestSettingsRepository
	blockRepo            repositories.BlockRepository
}

func NewQuestService(
	instanceRepo repositories.QuestRepository,
	instanceSettingsRepo repositories.QuestSettingsRepository,
	blockRepo repositories.BlockRepository,
) *QuestService {
	return &QuestService{
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		blockRepo:            blockRepo,
	}
}

// CreateQuest implements QuestService.
func (s *QuestService) CreateQuest(
	ctx context.Context,
	name string,
	user *models.User,
) (*models.Quest, error) {
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	if user == nil {
		return nil, ErrUserNotAuthenticated
	}

	instance := &models.Quest{
		Name:       name,
		UserID:     user.ID,
		IsTemplate: false,
		GameStructure: models.GameStructure{
			ID:             uuid.New().String(),
			Name:           "",
			Color:          "",
			Routing:        models.RouteStrategyFreeRoam,
			CompletionType: models.CompletionAll,
			IsRoot:         true,
			LocationIDs:    []string{},
			SubGroups: []models.GameStructure{
				{
					ID:             uuid.New().String(),
					Name:           "Locations",
					Color:          "primary",
					Routing:        models.RouteStrategyRandomised,
					CompletionType: models.CompletionAll,
					MaxNext:        3, //nolint:mnd // Default max next locations
					AutoAdvance:    true,
					IsRoot:         false,
					LocationIDs:    []string{},
					SubGroups:      []models.GameStructure{},
				},
			},
		},
	}

	if err := s.instanceRepo.Create(ctx, instance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}

	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	if err := s.instanceSettingsRepo.Create(ctx, settings); err != nil {
		return nil, fmt.Errorf("creating instance settings: %w", err)
	}

	// Create default start blocks
	if err := s.createDefaultStartBlocks(ctx, instance); err != nil {
		return nil, fmt.Errorf("creating default start blocks: %w", err)
	}

	// Create default finish blocks
	if err := s.createDefaultFinishBlocks(ctx, instance); err != nil {
		return nil, fmt.Errorf("creating default finish blocks: %w", err)
	}

	return instance, nil
}

// FindByUserID implements QuestService.
func (s *QuestService) FindByUserID(ctx context.Context, userID string) ([]models.Quest, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}
	return instances, nil
}

// FindQuestIDsForUser implements QuestService.
func (s *QuestService) FindQuestIDsForUser(ctx context.Context, userID string) ([]string, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}

	ids := make([]string, len(instances))
	for i, instance := range instances {
		ids[i] = instance.ID
	}
	return ids, nil
}

// GetByID finds an instance by ID.
func (s *QuestService) GetByID(ctx context.Context, id string) (*models.Quest, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting instance by ID: %w", err)
	}
	return instance, nil
}

// GetQuestSettings returns the settings for a quest.
func (s *QuestService) GetQuestSettings(
	ctx context.Context,
	questID string,
) (*models.QuestSettings, error) {
	settings, err := s.instanceSettingsRepo.GetByQuestID(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("getting instance settings: %w", err)
	}
	return settings, nil
}

// Update updates an instance.
func (s *QuestService) Update(ctx context.Context, instance *models.Quest) error {
	if instance == nil {
		return errors.New("instance cannot be nil")
	}

	if instance.Name == "" {
		return errors.New("name cannot be empty")
	}

	if err := s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("updating instance: %w", err)
	}

	return nil
}

const startInstructionsContent = `- Navigate to each location using the clues, maps, or directions provided.
- When you arrive, check in by scanning the QR code or following the link.
- Complete the activity at each stop.
- Continue moving through all locations and completing their activities until you reach the final checkpoint.
- Have fun exploring!`

const finishCongratulationsContent = `You’ve wrapped up the entire route. Thanks for being part of the adventure.`

// createDefaultStartBlocks creates the default blocks for an instance's start page.
func (s *QuestService) createDefaultStartBlocks(ctx context.Context, instance *models.Quest) error {
	startBlocks := []blocks.Block{
		// 1. Header block
		&blocks.HeaderBlock{
			BaseBlock: blocks.BaseBlock{Order: 0},
			Icon:      "map-pin-check-inside",
			TitleText: instance.Name,
			TitleSize: "large",
		},
		// 2. Game status alert
		&blocks.GameStatusAlertBlock{
			BaseBlock:        blocks.BaseBlock{Order: 1},
			ClosedMessage:    "This game is not yet open.",
			ScheduledMessage: "This game will start soon.",
			ShowCountdown:    true,
		},
		// 3. Divider - "How to play"
		&blocks.DividerBlock{
			BaseBlock: blocks.BaseBlock{Order: 2}, //nolint:mnd // Sequential ordering
			Title:     "How to play",
		},
		// 4. Markdown - Instructions content
		&blocks.MarkdownBlock{
			BaseBlock: blocks.BaseBlock{Order: 3}, //nolint:mnd // Sequential ordering
			Content:   startInstructionsContent,
		},
		// 5. Divider - "Team Info"
		&blocks.DividerBlock{
			BaseBlock: blocks.BaseBlock{Order: 4}, //nolint:mnd // Sequential ordering
			Title:     "Team Info",
		},
		// 6. Team name changer
		&blocks.TeamNameChangerBlock{
			BaseBlock:     blocks.BaseBlock{Order: 5}, //nolint:mnd // Sequential ordering
			BlockText:     "Set your team name",
			AllowChanging: true,
		},
		// 7. Start game button
		&blocks.StartGameButtonBlock{
			BaseBlock:           blocks.BaseBlock{Order: 6}, //nolint:mnd // Sequential ordering
			ScheduledButtonText: "Game starts soon...",
			ActiveButtonText:    "Start Game",
			ButtonStyle:         "primary",
		},
	}

	return s.blockRepo.BulkCreate(ctx, startBlocks, instance.ID, blocks.ContextStart)
}

// createDefaultFinishBlocks creates the default blocks for an instance's finish page.
func (s *QuestService) createDefaultFinishBlocks(ctx context.Context, instance *models.Quest) error {
	finishBlocks := []blocks.Block{
		// 1. Header block
		&blocks.HeaderBlock{
			BaseBlock: blocks.BaseBlock{Order: 0},
			Icon:      "party-popper",
			TitleText: "Congratulations!",
			TitleSize: "large",
		},
		// 2. Markdown - Congratulations text
		&blocks.MarkdownBlock{
			BaseBlock: blocks.BaseBlock{Order: 1},
			Content:   finishCongratulationsContent,
		},
	}

	return s.blockRepo.BulkCreate(ctx, finishBlocks, instance.ID, blocks.ContextFinish)
}
