package services

import (
	"context"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type CheckInService interface {
	// CompleteBlocks marks all blocks for a location as complete
	CompleteBlocks(ctx context.Context, teamCode string, locationID string) error
	// CheckIn logs a check in for a team at a location
	CheckIn(ctx context.Context, team models.Team, location models.Location, mustCheckOut bool, validationRequired bool) (models.CheckIn, error)
	// CheckOut logs a check out for a team at a location
	CheckOut(ctx context.Context, team *models.Team, location *models.Location) (models.CheckIn, error)
	// UpdateCheckIn updates an existing check-in
	UpdateCheckIn(ctx context.Context, checkIn *models.CheckIn) error
}

type checkInService struct {
	checkInRepo  repositories.CheckInRepository
	locationRepo repositories.LocationRepository
	teamRepo     repositories.TeamRepository
}

func NewCheckInService(
	checkInRepo repositories.CheckInRepository,
	locationRepo repositories.LocationRepository,
	teamRepo repositories.TeamRepository,
) CheckInService {
	return &checkInService{
		checkInRepo:  checkInRepo,
		locationRepo: locationRepo,
		teamRepo:     teamRepo,
	}
}

func (s *checkInService) CompleteBlocks(ctx context.Context, teamCode string, locationID string) error {
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
func (s *checkInService) CheckIn(ctx context.Context, team models.Team, location models.Location, mustCheckOut bool, validationRequired bool) (models.CheckIn, error) {
	scan, err := s.checkInRepo.LogCheckIn(ctx, team, location, mustCheckOut, validationRequired)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("logging check in: %w", err)
	}
	return scan, nil
}

// CheckOut logs a check out for a team at a location.
func (s *checkInService) CheckOut(ctx context.Context, team *models.Team, location *models.Location) (models.CheckIn, error) {
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

// UpdateCheckIn updates an existing check-in.
func (s *checkInService) UpdateCheckIn(ctx context.Context, checkIn *models.CheckIn) error {
	return s.checkInRepo.Update(ctx, checkIn)
}
