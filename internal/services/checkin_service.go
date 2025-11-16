package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

const (
	bonusFirstVisit  = 1.0
	bonusSecondVisit = 0.5
	bonusThirdVisit  = 0.2
)

type LocationStatsService interface {
	IncrementVisitors(ctx context.Context, location *models.Location) error
	DecrementVisitors(ctx context.Context, location *models.Location) error
}

type CheckInService struct {
	checkInRepo          repositories.CheckInRepository
	locationRepo         repositories.LocationRepository
	teamRepo             repositories.TeamRepository
	blockService         *BlockService
	locationStatsService LocationStatsService
	navigationService    *NavigationService
}

func NewCheckInService(
	checkInRepo repositories.CheckInRepository,
	locationRepo repositories.LocationRepository,
	teamRepo repositories.TeamRepository,
	locationStatsService LocationStatsService,
	navigationService *NavigationService,
	blockService *BlockService,
) *CheckInService {
	return &CheckInService{
		checkInRepo:          checkInRepo,
		locationRepo:         locationRepo,
		teamRepo:             teamRepo,
		locationStatsService: locationStatsService,
		navigationService:    navigationService,
		blockService:         blockService,
	}
}

func (s *CheckInService) CheckIn(ctx context.Context, team *models.Team, locationCode string) error {
	// Load team relations
	err := s.teamRepo.LoadRelations(ctx, team)
	if err != nil {
		return fmt.Errorf("loading relations: %w", err)
	}

	// A team may not check in if they must check out at a different location
	if team.MustCheckOut != "" && locationCode != team.MustCheckOut {
		return ErrAlreadyCheckedIn
	}

	// Find the location
	location, err := s.locationRepo.GetByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		return fmt.Errorf("%w: finding location: %w", ErrLocationNotFound, err)
	}

	// The team relations loaded above include Instance and Instance.Settings
	// Copy the instance settings to the location for bonus points calculation
	location.Instance = team.Instance

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

	valid, err := s.navigationService.IsValidLocation(ctx, team, locationCode)
	if err != nil {
		return fmt.Errorf("checking if location is valid: %w", err)
	}
	if !valid {
		return errors.New("location not valid for team")
	}

	// Check if any blocks require validation (e.g. a checklist)
	validationRequired, err := s.blockService.CheckValidationRequiredForLocation(ctx, location.ID)
	if err != nil {
		return fmt.Errorf("checking if validation is required: %w", err)
	}

	// Calculate the points to award
	var pointsForCheckInRecord int
	var bonusPoints int

	if team.Instance.Settings.MustCheckOut {
		// Check-in-and-out mode: bonus points awarded immediately, base points on completion
		if location.Instance.Settings.EnableBonusPoints {
			// Calculate bonus points based on visit count
			switch location.TotalVisits {
			case 0:
				bonusPoints = location.Points // First visit gets +100% bonus (2x total)

			case 1:
				bonusPoints = int(
					float64(location.Points) * bonusSecondVisit,
				) // Second visit gets +50% bonus (1.5x total)
			//nolint:mnd // Magic numbers for bonus multipliers
			case 2:
				bonusPoints = int(
					float64(location.Points) * bonusThirdVisit,
				) // Third visit gets +20% bonus (1.2x total)
			default:
				bonusPoints = 0 // No bonus for later visits
			}
		}
		// For CheckIn record: only bonus points are recorded (base points awarded at checkout)
		pointsForCheckInRecord = bonusPoints
		// Award bonus points to team immediately
		team.Points += bonusPoints
		team.MustCheckOut = location.ID
	} else {
		// Check-in-only mode: full points awarded immediately
		if location.Instance.Settings.EnableBonusPoints {
			// Calculate total points with bonus
			switch location.TotalVisits {
			case 0:
				pointsForCheckInRecord = location.Points * (1 + bonusFirstVisit) // First visit gets double points

			case 1:
				pointsForCheckInRecord = int(float64(location.Points) * (1 + bonusSecondVisit)) // Second visit gets 1.5x points
			//nolint:mnd // Magic numbers for bonus multipliers
			case 2:
				pointsForCheckInRecord = int(float64(location.Points) * (1 + bonusThirdVisit)) // Third visit gets 1.2x points
			default:
				pointsForCheckInRecord = location.Points // Regular points for all other visits
			}
		} else {
			pointsForCheckInRecord = location.Points
		}
		// Award full points to team immediately
		team.Points += pointsForCheckInRecord
	}

	// Create a copy of the location with the calculated points for the CheckIn record
	locationForCheckIn := *location
	locationForCheckIn.Points = pointsForCheckInRecord

	// Log the check in with the correct points
	_, err = s.checkIn(ctx, *team, locationForCheckIn, team.Instance.Settings.MustCheckOut, validationRequired)
	if err != nil {
		return fmt.Errorf("logging scan: %w", err)
	}

	err = s.locationStatsService.IncrementVisitors(ctx, location)
	if err != nil {
		return fmt.Errorf("incrementing visitor stats: %w", err)
	}

	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		return fmt.Errorf("updating team: %w", err)
	}

	return nil
}

func (s *CheckInService) CheckOut(ctx context.Context, team *models.Team, locationCode string) error {
	location, err := s.locationRepo.GetByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		return fmt.Errorf("%w: finding location: %w", ErrLocationNotFound, err)
	}

	err = s.teamRepo.LoadRelations(ctx, team)
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
	unfinishedCheckIn, err := s.blockService.CheckValidationRequiredForCheckIn(ctx, location.ID, team.Code)
	if err != nil {
		return fmt.Errorf("checking if validation is required: %w", err)
	}

	if unfinishedCheckIn {
		return ErrUnfinishedCheckIn
	}

	// Copy the team's instance settings to the location for consistency
	location.Instance = team.Instance

	// Award base points on checkout completion
	team.Points += location.Points

	// Log the scan out and get the updated CheckIn record
	checkIn, err := s.checkOut(ctx, team, location)
	if err != nil {
		return fmt.Errorf("logging scan out: %w", err)
	}

	// Update the CheckIn record to include the base points in addition to any bonus points
	// This ensures the CheckIn record shows the total points earned from this location
	checkIn.Points += location.Points
	err = s.checkInRepo.Update(ctx, &checkIn)
	if err != nil {
		return fmt.Errorf("updating check in points: %w", err)
	}

	// Update team with the awarded points
	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		return fmt.Errorf("updating team points: %w", err)
	}

	return nil
}

func (s *CheckInService) CompleteBlocks(ctx context.Context, teamCode string, locationID string) error {
	checkIn, err := s.checkInRepo.FindCheckInByTeamAndLocation(ctx, teamCode, locationID)
	if err != nil {
		return fmt.Errorf("finding check in: %w", err)
	}

	// If the check in is already complete, return early
	if checkIn.BlocksCompleted {
		return nil
	}

	checkIn.BlocksCompleted = true
	err = s.checkInRepo.Update(ctx, checkIn)
	if err != nil {
		return fmt.Errorf("updating check in: %w", err)
	}

	return nil
}

// CheckIn logs a check in for a team at a location.
func (s *CheckInService) checkIn(
	ctx context.Context,
	team models.Team,
	location models.Location,
	mustCheckOut bool,
	validationRequired bool,
) (models.CheckIn, error) {
	scan, err := s.checkInRepo.LogCheckIn(ctx, team, location, mustCheckOut, validationRequired)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("logging check in: %w", err)
	}
	return scan, nil
}

// CheckOut logs a check out for a team at a location.
func (s *CheckInService) checkOut(
	ctx context.Context,
	team *models.Team,
	location *models.Location,
) (models.CheckIn, error) {
	scan, err := s.checkInRepo.LogCheckOut(ctx, team, location)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("checking out: %w", err)
	}

	// Update location statistics
	location.AvgDuration =
		(location.AvgDuration*float64(location.TotalVisits) +
			scan.TimeOut.Sub(scan.TimeIn).Seconds()) /
			float64(location.TotalVisits+1)
	location.CurrentCount--
	err = s.locationRepo.Update(ctx, location)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("updating location: %w", err)
	}

	// Update team
	team.MustCheckOut = ""
	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("updating team: %w", err)
	}

	return scan, nil
}

func (s *CheckInService) ValidateAndUpdateBlockState(
	ctx context.Context,
	team models.Team,
	data map[string][]string,
) (blocks.PlayerState, blocks.Block, error) {
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
		block, err = s.blockService.GetByBlockID(ctx, blockID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block in preview mode: %w", err)
		}

		state, err = s.blockService.NewMockBlockState(ctx, blockID, team.Code)
		if err != nil {
			return nil, nil, fmt.Errorf("creating mock state in preview mode: %w", err)
		}
	} else {
		// In regular mode, get the existing block and state
		block, state, err = s.blockService.GetBlockWithStateByBlockIDAndTeamCode(ctx, blockID, team.Code)
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
		state, err = s.blockService.UpdateState(ctx, state)
		if err != nil {
			return nil, nil, fmt.Errorf("updating block state: %w", err)
		}
	}

	// Only award points and update check-ins in regular mode, not preview mode
	if !isPreview && state.IsComplete() {
		team.Points += block.GetPoints()
		err = s.teamRepo.Update(ctx, &team)
		if err != nil {
			return nil, nil, fmt.Errorf("awarding points: %w", err)
		}

		// Update the check in all blocks have been completed
		unfinishedCheckIn, checkErr := s.blockService.CheckValidationRequiredForCheckIn(
			ctx,
			block.GetLocationID(),
			team.Code,
		)
		if checkErr != nil {
			return nil, nil, fmt.Errorf("checking if validation is required: %w", checkErr)
		}

		if !unfinishedCheckIn {
			err = s.CompleteBlocks(ctx, team.Code, block.GetLocationID())
			if err != nil {
				return nil, nil, fmt.Errorf("completing blocks: %w", err)
			}
		}
	}

	return state, block, nil
}
