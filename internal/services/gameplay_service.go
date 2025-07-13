package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nathanhollows/Rapua/v3/blocks"
	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
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
	CheckGameStatus(ctx context.Context, team *models.Team) *ServiceResponse
	GetTeamByCode(ctx context.Context, teamCode string) (*models.Team, error)
	GetMarkerByCode(ctx context.Context, locationCode string) *ServiceResponse
	StartPlaying(ctx context.Context, teamCode, customTeamName string) *ServiceResponse
	// CheckIn checks a team in at a location
	// It also manages the points and mustScanOut fields
	// As well as checking if any blocks must be completed
	ValidateAndUpdateBlockState(ctx context.Context, team models.Team, data map[string][]string) (blocks.PlayerState, blocks.Block, error)
}

type gameplayService struct {
	CheckInService   CheckInService
	LocationService  LocationService
	TeamService      TeamService
	BlockService     BlockService
	MarkerRepository repositories.MarkerRepository
}

func NewGameplayService(
	checkInService CheckInService,
	locationService LocationService,
	teamService TeamService,
	blockService BlockService,
	markerRepository repositories.MarkerRepository,
) GameplayService {
	return &gameplayService{
		CheckInService:   checkInService,
		LocationService:  locationService,
		TeamService:      teamService,
		BlockService:     blockService,
		MarkerRepository: markerRepository,
	}
}

// GetGameStatus returns the current status of the game.
func (s *gameplayService) CheckGameStatus(ctx context.Context, team *models.Team) (response *ServiceResponse) {
	response = &ServiceResponse{}
	response.Data = make(map[string]interface{})

	// Load the instance
	err := s.TeamService.LoadRelation(ctx, team, "Instance")
	if err != nil {
		response.Error = fmt.Errorf("loading instance: %w", err)
		return response
	}

	status := team.Instance.GetStatus()
	response.Data["status"] = status
	return response
}

func (s *gameplayService) GetTeamByCode(ctx context.Context, teamCode string) (*models.Team, error) {
	teamCode = strings.TrimSpace(strings.ToUpper(teamCode))
	team, err := s.TeamService.FindTeamByCode(ctx, teamCode)
	if err != nil {
		return nil, fmt.Errorf("GetTeamStatus: %w", err)
	}
	return team, nil
}

func (s *gameplayService) GetMarkerByCode(ctx context.Context, locationCode string) (response *ServiceResponse) {
	response = &ServiceResponse{}
	response.Data = make(map[string]interface{})

	locationCode = strings.TrimSpace(strings.ToUpper(locationCode))
	marker, err := s.MarkerRepository.GetByCode(ctx, locationCode)
	if err != nil {
		response.Error = fmt.Errorf("GetLocationByCode finding marker: %w", err)
		return response
	}
	response.Data["marker"] = marker
	return response
}

func (s *gameplayService) StartPlaying(ctx context.Context, teamCode, customTeamName string) (response *ServiceResponse) {
	response = &ServiceResponse{}
	response.Data = make(map[string]interface{})

	team, err := s.TeamService.FindTeamByCode(ctx, teamCode)
	if err != nil {
		response.Error = fmt.Errorf("StartPlaying find team: %w", err)
		response.AddFlashMessage(*flash.NewError("Team not found. Please double check the code and try again."))
		return response
	}

	// Update team with custom name if provided
	if !team.HasStarted || customTeamName != "" {
		team.Name = customTeamName
		team.HasStarted = true
		err = s.TeamService.Update(ctx, team)
		if err != nil {
			response.Error = fmt.Errorf("StartPlaying update team: %w", err)
			response.AddFlashMessage(*flash.NewError("Something went wrong. Please try again."))
			return response
		}
	}

	response.Data["team"] = team
	response.AddFlashMessage(*flash.NewSuccess("You have started the game!"))
	return response
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
