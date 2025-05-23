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
	SuggestNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error)
	// CheckIn checks a team in at a location
	// It also manages the points and mustScanOut fields
	// As well as checking if any blocks must be completed
	CheckIn(ctx context.Context, team *models.Team, locationCode string) error
	CheckOut(ctx context.Context, team *models.Team, locationCode string) error
	CheckValidLocation(ctx context.Context, team *models.Team, locationCode string) (bool, error)
	ValidateAndUpdateBlockState(ctx context.Context, team models.Team, data map[string][]string) (blocks.PlayerState, blocks.Block, error)
}

type gameplayService struct {
	CheckInService    CheckInService
	LocationService   LocationService
	TeamService       TeamService
	BlockService      BlockService
	NavigationService NavigationService
	MarkerRepository  repositories.MarkerRepository
}

func NewGameplayService(
	checkInService CheckInService,
	locationService LocationService,
	teamService TeamService,
	blockService BlockService,
	navigationService NavigationService,
	markerRepository repositories.MarkerRepository,
) GameplayService {
	return &gameplayService{
		CheckInService:    checkInService,
		LocationService:   locationService,
		TeamService:       teamService,
		BlockService:      blockService,
		NavigationService: navigationService,
		MarkerRepository:  markerRepository,
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

func (s *gameplayService) SuggestNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	// Populate the team with the necessary data
	err := s.TeamService.LoadRelations(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("loading relations: %w", err)
	}

	// Suggest the next locations for the team
	locations, err := s.NavigationService.DetermineNextLocations(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("determining next locations: %w", err)
	}

	for i := range locations {
		err := s.LocationService.LoadRelations(ctx, &locations[i])
		if err != nil {
			return nil, fmt.Errorf("loading relations for location: %w", err)
		}
	}

	return locations, nil
}

func (s *gameplayService) CheckIn(ctx context.Context, team *models.Team, locationCode string) error {
	// Load team relations
	err := s.TeamService.LoadRelations(ctx, team)
	if err != nil {
		return fmt.Errorf("loading relations: %w", err)
	}

	// A team may not check in if they must check out at a different location
	if team.MustCheckOut != "" && locationCode != team.MustCheckOut {
		return ErrAlreadyCheckedIn
	}

	// Find the location
	location, err := s.LocationService.GetByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		return fmt.Errorf("%w: finding location: %w", ErrLocationNotFound, err)
	}

	// A team may not check in if they have previously checked in at this location
	scanned := false
	for _, s := range team.CheckIns {
		if s.LocationID == location.ID {
			scanned = true
			break
		}
	}
	if scanned {
		return ErrAlreadyCheckedIn
	}

	valid, err := s.NavigationService.CheckValidLocation(ctx, team, &team.Instance.Settings, locationCode)
	if err != nil {
		return fmt.Errorf("checking if location is valid: %w", err)
	}
	if !valid {
		return errors.New("location not valid for team")
	}

	// Check if any blocks require validation (e.g. a checklist)
	validationRequired, err := s.BlockService.CheckValidationRequiredForLocation(ctx, location.ID)
	if err != nil {
		return fmt.Errorf("checking if validation is required: %w", err)
	}

	// Log the check in
	mustCheckOut := team.Instance.Settings.CompletionMethod == models.CheckInAndOut
	_, err = s.CheckInService.CheckIn(ctx, *team, *location, mustCheckOut, validationRequired)
	if err != nil {
		return fmt.Errorf("logging scan: %w", err)
	}

	err = s.LocationService.IncrementVisitorStats(ctx, location)
	if err != nil {
		return fmt.Errorf("incrementing visitor stats: %w", err)
	}

	// Points are only added if the team does not need to scan out
	// If the team must check out, the location is saved to the team
	if mustCheckOut {
		team.MustCheckOut = location.ID
	} else {
		team.Points += location.Points
	}
	err = s.TeamService.Update(ctx, team)
	if err != nil {
		return fmt.Errorf("updating team: %w", err)
	}

	return nil
}

func (s *gameplayService) CheckOut(ctx context.Context, team *models.Team, locationCode string) error {
	location, err := s.LocationService.GetByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		return fmt.Errorf("%w: finding location: %w", ErrLocationNotFound, err)
	}

	err = s.TeamService.LoadRelations(ctx, team)
	if err != nil {
		return fmt.Errorf("loading relations: %w", err)
	}

	// Check if the team must scan out
	if team.MustCheckOut == "" {
		return ErrUnecessaryCheckOut
	} else if team.MustCheckOut != location.ID {
		return ErrCheckOutAtWrongLocation
	}

	// Check if all blocks are completed
	unfinishedCheckIn, err := s.BlockService.CheckValidationRequiredForCheckIn(ctx, location.ID, team.Code)
	if err != nil {
		return fmt.Errorf("checking if validation is required: %w", err)
	}

	if unfinishedCheckIn {
		return ErrUnfinishedCheckIn
	}

	// Log the scan out
	_, err = s.CheckInService.CheckOut(ctx, team, location)
	if err != nil {
		return fmt.Errorf("logging scan out: %w", err)
	}

	return nil
}

// CheckLocation checks if the location is valid for the team to check in.
func (s *gameplayService) CheckValidLocation(ctx context.Context, team *models.Team, locationCode string) (bool, error) {
	if team.Instance.ID == "" {
		return false, ErrInstanceNotFound
	}

	if team.Instance.Settings.InstanceID == "" {
		return false, ErrInstanceSettingsNotFound
	}

	if len(team.Instance.Locations) == 0 {
		return false, ErrLocationNotFound
	}

	valid, err := s.NavigationService.CheckValidLocation(ctx, team, &team.Instance.Settings, locationCode)
	if err != nil {
		return false, fmt.Errorf("checking if location is valid: %w", err)
	}

	return valid, nil
}

func (s *gameplayService) ValidateAndUpdateBlockState(ctx context.Context, team models.Team, data map[string][]string) (blocks.PlayerState, blocks.Block, error) {
	blockID := data["block"][0]
	if blockID == "" {
		return nil, nil, errors.New("blockID must be set")
	}

	block, state, err := s.BlockService.GetBlockWithStateByBlockIDAndTeamCode(ctx, blockID, team.Code)
	if err != nil {
		return nil, nil, fmt.Errorf("getting block with state: %w", err)
	}

	if state == nil {
		return nil, nil, errors.New("block state not found")
	}

	// Check if we're in preview mode - preview mode should allow resubmission for testing
	isPreview := ctx.Value(contextkeys.PreviewKey) != nil
	
	// Returning early here prevents the block from being updated
	// And points from being added to the team multiple times
	// But in preview mode, we want to allow resubmission for testing
	if state.IsComplete() && !isPreview {
		return state, block, nil
	}

	// Validate the block
	state, err = block.ValidatePlayerInput(state, data)
	if err != nil {
		return nil, nil, fmt.Errorf("validating block: %w", err)
	}

	state, err = s.BlockService.UpdateState(ctx, state)
	if err != nil {
		return nil, nil, fmt.Errorf("updating block state: %w", err)
	}

	// Assign points on completion
	if !state.IsComplete() {
		return state, block, nil
	}
	err = s.TeamService.AwardPoints(ctx, &team, block.GetPoints(), fmt.Sprint("Completed block ", block.GetName()))
	if err != nil {
		return nil, nil, fmt.Errorf("awarding points: %w", err)
	}

	// Update the check in all blocks have been completed
	unfinishedCheckIn, err := s.BlockService.CheckValidationRequiredForCheckIn(ctx, block.GetLocationID(), team.Code)
	if err != nil {
		return nil, nil, fmt.Errorf("checking if validation is required: %w", err)
	}

	if unfinishedCheckIn {
		return state, block, nil
	}

	err = s.CheckInService.CompleteBlocks(ctx, team.Code, block.GetLocationID())
	if err != nil {
		return nil, nil, fmt.Errorf("completing blocks: %w", err)
	}

	return state, block, nil
}
