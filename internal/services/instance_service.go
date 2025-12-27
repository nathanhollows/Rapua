package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

type InstanceService struct {
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	blockRepo            repositories.BlockRepository
}

func NewInstanceService(
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	blockRepo repositories.BlockRepository,
) *InstanceService {
	return &InstanceService{
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		blockRepo:            blockRepo,
	}
}

// CreateInstance implements InstanceService.
func (s *InstanceService) CreateInstance(
	ctx context.Context,
	name string,
	user *models.User,
) (*models.Instance, error) {
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	if user == nil {
		return nil, ErrUserNotAuthenticated
	}

	instance := &models.Instance{
		Name:       name,
		UserID:     user.ID,
		IsTemplate: false,
		GameStructure: models.GameStructure{
			ID:             uuid.New().String(),
			Name:           "",
			Color:          "",
			Routing:        models.RouteStrategyFreeRoam,
			Navigation:     models.NavigationDisplayMap,
			CompletionType: models.CompletionAll,
			IsRoot:         true,
			LocationIDs:    []string{},
			SubGroups: []models.GameStructure{
				{
					ID:             uuid.New().String(),
					Name:           "Locations",
					Color:          "primary",
					Routing:        models.RouteStrategyRandom,
					Navigation:     models.NavigationDisplayCustom,
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

	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
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

// FindByUserID implements InstanceService.
func (s *InstanceService) FindByUserID(ctx context.Context, userID string) ([]models.Instance, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}
	return instances, nil
}

// FindInstanceIDsForUser implements InstanceService.
func (s *InstanceService) FindInstanceIDsForUser(ctx context.Context, userID string) ([]string, error) {
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
func (s *InstanceService) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting instance by ID: %w", err)
	}
	return instance, nil
}

// GetInstanceSettings returns the settings for an instance.
func (s *InstanceService) GetInstanceSettings(
	ctx context.Context,
	instanceID string,
) (*models.InstanceSettings, error) {
	settings, err := s.instanceSettingsRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("getting instance settings: %w", err)
	}
	return settings, nil
}

// Update updates an instance.
func (s *InstanceService) Update(ctx context.Context, instance *models.Instance) error {
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
func (s *InstanceService) createDefaultStartBlocks(ctx context.Context, instance *models.Instance) error {
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
func (s *InstanceService) createDefaultFinishBlocks(ctx context.Context, instance *models.Instance) error {
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
