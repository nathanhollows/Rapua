package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nathanhollows/Rapua/v3/blocks"
	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

// Define errors.
var (
	ErrTeamNotFound             = errors.New("team not found")
	ErrLocationNotFound         = errors.New("location not found")
	ErrCheckOutAtWrongLocation  = errors.New("team is not at the correct location to check out")
	ErrUnfinishedCheckIn        = errors.New("unfinished check in")
	ErrAlreadyCheckedIn         = errors.New("player has already scanned in")
	ErrUnecessaryCheckOut       = errors.New("player does not need to scan out")
	ErrInstanceSettingsNotFound = errors.New("instance settings not found")
)

type GameplayService interface {
	GetTeamByCode(ctx context.Context, teamCode string) (*models.Team, error)
	GetMarkerByCode(ctx context.Context, locationCode string) (models.Marker, error)
	StartPlaying(ctx context.Context, teamCode, customTeamName string) error
	ValidateAndUpdateBlockState(ctx context.Context, team models.Team, data map[string][]string) (blocks.PlayerState, blocks.Block, error)
}

type gameplayService struct {
	CheckInService   CheckInService
	TeamService      TeamService
	BlockService     BlockService
	MarkerRepository repositories.MarkerRepository
}

func NewGameplayService(
	checkInService CheckInService,
	teamService TeamService,
	blockService BlockService,
	markerRepository repositories.MarkerRepository,
) GameplayService {
	return &gameplayService{
		CheckInService:   checkInService,
		TeamService:      teamService,
		BlockService:     blockService,
		MarkerRepository: markerRepository,
	}
}

func (s *gameplayService) GetTeamByCode(ctx context.Context, teamCode string) (*models.Team, error) {
	teamCode = strings.TrimSpace(strings.ToUpper(teamCode))
	team, err := s.TeamService.FindTeamByCode(ctx, teamCode)
	if err != nil {
		return nil, fmt.Errorf("GetTeamStatus: %w", err)
	}
	return team, nil
}

func (s *gameplayService) GetMarkerByCode(ctx context.Context, locationCode string) (models.Marker, error) {
	locationCode = strings.TrimSpace(strings.ToUpper(locationCode))
	marker, err := s.MarkerRepository.GetByCode(ctx, locationCode)
	if err != nil {
		return models.Marker{}, fmt.Errorf("GetMarkerByCode: %w", err)
	}
	return *marker, nil
}

func (s *gameplayService) StartPlaying(ctx context.Context, teamCode, customTeamName string) error {
	team, err := s.TeamService.FindTeamByCode(ctx, teamCode)
	if err != nil {
		return ErrTeamNotFound
	}

	// Update team with custom name if provided
	if !team.HasStarted || customTeamName != "" {
		team.Name = customTeamName
		team.HasStarted = true
		err = s.TeamService.Update(ctx, team)
		if err != nil {
			return fmt.Errorf("updating team: %w", err)
		}
	}

	return nil
}

func (s *gameplayService) ValidateAndUpdateBlockState(ctx context.Context, team models.Team, data map[string][]string) (blocks.PlayerState, blocks.Block, error) {
	blockID := data["block"][0]
	if blockID == "" {
		return nil, nil, errors.New("blockID must be set")
	}

	// Check if we're in preview mode - preview mode should use fresh mock state
	isPreview := ctx.Value(contextkeys.PreviewKey) != nil

	var block blocks.Block
	var state blocks.PlayerState
	var err error

	if isPreview {
		// In preview mode, always get a fresh block and create a new mock state
		block, err = s.BlockService.GetByBlockID(ctx, blockID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block in preview mode: %w", err)
		}

		state, err = s.BlockService.NewMockBlockState(ctx, blockID, team.Code)
		if err != nil {
			return nil, nil, fmt.Errorf("creating mock state in preview mode: %w", err)
		}
	} else {
		// In regular mode, get the existing block and state
		block, state, err = s.BlockService.GetBlockWithStateByBlockIDAndTeamCode(ctx, blockID, team.Code)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block with state: %w", err)
		}
	}

	if block == nil {
		return nil, nil, errors.New("block not found")
	}

	if state == nil {
		return nil, nil, errors.New("block state not found")
	}

	// In regular mode, return early if already complete to prevent duplicate points
	// Preview mode always uses fresh state so this check is not needed
	if !isPreview && state.IsComplete() {
		return state, block, nil
	}

	// Validate the block
	state, err = block.ValidatePlayerInput(state, data)
	if err != nil {
		return nil, nil, fmt.Errorf("validating block: %w", err)
	}

	// Only persist state changes in regular mode, not in preview mode
	if !isPreview {
		state, err = s.BlockService.UpdateState(ctx, state)
		if err != nil {
			return nil, nil, fmt.Errorf("updating block state: %w", err)
		}
	}

	// Only award points and update check-ins in regular mode, not preview mode
	if !isPreview && state.IsComplete() {
		err = s.TeamService.AwardPoints(ctx, &team, block.GetPoints(), fmt.Sprint("Completed block ", block.GetName()))
		if err != nil {
			return nil, nil, fmt.Errorf("awarding points: %w", err)
		}

		// Update the check in all blocks have been completed
		unfinishedCheckIn, err := s.BlockService.CheckValidationRequiredForCheckIn(ctx, block.GetLocationID(), team.Code)
		if err != nil {
			return nil, nil, fmt.Errorf("checking if validation is required: %w", err)
		}

		if !unfinishedCheckIn {
			err = s.CheckInService.CompleteBlocks(ctx, team.Code, block.GetLocationID())
			if err != nil {
				return nil, nil, fmt.Errorf("completing blocks: %w", err)
			}
		}
	}

	return state, block, nil
}
