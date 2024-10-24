package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nathanhollows/Rapua/internal/models"
	"github.com/nathanhollows/Rapua/internal/repositories"
)

type LocationService interface {
	FindLocation(ctx context.Context, locationID string) (*models.Location, error)
	FindLocationByInstanceAndCode(ctx context.Context, instanceID string, code string) (*models.Location, error)
	LoadCluesForLocation(ctx context.Context, location *models.Location) error
	LoadCluesForLocations(ctx context.Context, locations *models.Locations) error
	IncrementVisitorStats(ctx context.Context, location *models.Location) error
}

type locationService struct {
	locationRepo repositories.LocationRepository
	clueRepo     repositories.ClueRepository
}

// NewLocationService creates a new instance of LocationService
func NewLocationService(clueRepo repositories.ClueRepository) LocationService {
	return locationService{
		clueRepo:     clueRepo,
		locationRepo: repositories.NewLocationRepository(),
	}
}

// FindLocation finds a location by ID
func (s locationService) FindLocation(ctx context.Context, locationID string) (*models.Location, error) {
	location, err := s.locationRepo.FindLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("finding location: %v", err)
	}
	return location, nil
}

// FindLocationByInstanceAndCode finds a location by instance and code
func (s locationService) FindLocationByInstanceAndCode(ctx context.Context, instanceID string, code string) (*models.Location, error) {
	location, err := s.locationRepo.FindLocationByInstanceAndCode(ctx, instanceID, code)
	if err != nil {
		return nil, fmt.Errorf("finding location by instance and code: %v", err)
	}
	return location, nil
}

// LoadCluesForLocation loads the clues for a specific location if they are not already loaded
func (s locationService) LoadCluesForLocation(ctx context.Context, location *models.Location) error {
	if len(location.Clues) == 0 {
		clues, err := s.clueRepo.FindCluesByLocation(ctx, location.ID)
		if err != nil {
			slog.Error("error loading clues for location", "locationID", location.ID, "err", err)
			return err
		}
		location.Clues = clues
	}
	return nil
}

// LoadCluesForLocations loads the clues for all given locations if they are not already loaded
func (s locationService) LoadCluesForLocations(ctx context.Context, locations *models.Locations) error {
	for i := range *locations {
		err := s.LoadCluesForLocation(ctx, &(*locations)[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// Update visitor stats for a location
func (s locationService) IncrementVisitorStats(ctx context.Context, location *models.Location) error {
	location.CurrentCount++
	location.TotalVisits++
	return s.locationRepo.Update(ctx, location)
}
