package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"golang.org/x/exp/rand"
)

var (
	ErrAllLocationsVisited = errors.New("all locations visited")
	ErrInstanceNotFound    = errors.New("instance not found")
)

type NavigationService struct {
	locationRepo repositories.LocationRepository
	teamRepo     repositories.TeamRepository
}

// NewNavigationService creates a new instance of NavigationService.
func NewNavigationService(locationRepo repositories.LocationRepository, teamRepo repositories.TeamRepository) *NavigationService {
	return &NavigationService{
		locationRepo: locationRepo,
		teamRepo:     teamRepo,
	}
}

// IsValidLocation checks if the location code is valid for the team to check in to.
func (s *NavigationService) IsValidLocation(ctx context.Context, team *models.Team, markerID string) (bool, error) {
	if err := s.validateTeamState(team); err != nil {
		return false, err
	}

	// Find valid locations (without loading full relations)
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return false, fmt.Errorf("determine next valid locations: %w", err)
	}

	// Check if the location code is valid
	markerID = s.normalizeMarkerID(markerID)
	for _, loc := range locations {
		if loc.MarkerID == markerID {
			return true, nil
		}
	}
	return false, fmt.Errorf("code %s is not a valid next location", markerID)
}

// GetNextLocations returns the next locations for the team to visit with full relations loaded.
func (s *NavigationService) GetNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	// Load team relations if not already loaded
	if err := s.ensureTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	// Get the core locations
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return nil, err
	}

	// Load full relations for each location
	for i := range locations {
		if err := s.locationRepo.LoadRelations(ctx, &locations[i]); err != nil {
			return nil, fmt.Errorf("loading relations for location: %w", err)
		}
	}

	return locations, nil
}

// getUnvisitedLocations returns a list of locations that the team has not visited.
func (s *NavigationService) getUnvisitedLocations(_ context.Context, team *models.Team) []models.Location {
	unvisited := make([]models.Location, 0, len(team.Instance.Locations))

	for _, location := range team.Instance.Locations {
		if !s.HasVisited(team.CheckIns, location.ID) {
			unvisited = append(unvisited, location)
		}
	}

	return unvisited
}

// getOrderedLocations returns the next location in the defined order.
func (s *NavigationService) getOrderedLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	unvisited := s.getUnvisitedLocations(ctx, team)
	if len(unvisited) == 0 {
		return nil, ErrAllLocationsVisited
	}

	// Find the location with the lowest order value
	nextLocation := unvisited[0]
	for _, location := range unvisited[1:] {
		if location.Order < nextLocation.Order {
			nextLocation = location
		}
	}

	return []models.Location{nextLocation}, nil
}

// getRandomLocations returns random locations for the team to visit.
// This function uses the team code as a seed for the random number generator.
// Process:
// 1. Shuffle the list of all locations deterministically based on team code,
// 2. Select the first n unvisited locations from the shuffled list,
// 3. Return these locations ensuring the order is consistent across refreshes,
// 3. Return these locations ensuring the order is consistent across refreshes.
func (s *NavigationService) getRandomLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	allLocations := team.Instance.Locations
	if len(allLocations) == 0 {
		return nil, errors.New("no locations found")
	}

	unvisited := s.getUnvisitedLocations(ctx, team)
	if len(unvisited) == 0 {
		return []models.Location{}, ErrAllLocationsVisited
	}

	// Seed the random number generator with the team code to ensure deterministic shuffling
	seed := uint64(0)
	for _, char := range team.Code {
		seed += uint64(char)
	}
	rand.Seed(seed)

	// We shuffle the list of all locations to ensure randomness
	// even when the team has visited some locations
	shuffledLocations := make([]models.Location, len(allLocations))
	copy(shuffledLocations, allLocations)
	rand.Shuffle(len(shuffledLocations), func(i, j int) {
		shuffledLocations[i], shuffledLocations[j] = shuffledLocations[j], shuffledLocations[i]
	})

	// Select the first n unvisited locations from the shuffled list
	n := team.Instance.Settings.MaxNextLocations
	selectedLocations := []models.Location{}
	for _, loc := range shuffledLocations {
		if !s.HasVisited(team.CheckIns, loc.ID) {
			selectedLocations = append(selectedLocations, loc)
			if len(selectedLocations) >= n {
				break
			}
		}
	}

	if len(selectedLocations) == 0 {
		return nil, ErrAllLocationsVisited
	}

	return selectedLocations, nil
}

// getFreeRoamLocations returns a list of locations for free roam mode. This
// function returns all locations in the instance for the team to visit.
func (s *NavigationService) getFreeRoamLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	unvisited := s.getUnvisitedLocations(ctx, team)

	if len(unvisited) == 0 {
		return nil, ErrAllLocationsVisited
	}

	return unvisited, nil
}

// HasVisited returns true if the team has visited the location.
func (s *NavigationService) HasVisited(checkins []models.CheckIn, locationID string) bool {
	for _, checkin := range checkins {
		if checkin.LocationID == locationID {
			return true
		}
	}
	return false
}

// determineNextLocations is the core logic for finding next locations without relation loading.
func (s *NavigationService) determineNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	if err := s.validateTeamState(team); err != nil {
		return nil, err
	}

	// Check if the team has visited all locations
	if len(team.CheckIns) == len(team.Instance.Locations) {
		return nil, ErrAllLocationsVisited
	}

	// Determine the next locations based on the navigation mode
	switch team.Instance.Settings.NavigationMode {
	case models.OrderedNav:
		return s.getOrderedLocations(ctx, team)
	case models.RandomNav:
		return s.getRandomLocations(ctx, team)
	case models.FreeRoamNav:
		return s.getFreeRoamLocations(ctx, team)
	}

	return nil, errors.New("invalid navigation mode")
}

// validateTeamState checks if team has required relations loaded.
func (s *NavigationService) validateTeamState(team *models.Team) error {
	if team.Instance.ID == "" {
		return ErrInstanceNotFound
	}
	if team.Instance.Settings.InstanceID == "" {
		return ErrInstanceSettingsNotFound
	}
	if len(team.Instance.Locations) == 0 {
		return ErrLocationNotFound
	}
	return nil
}

// ensureTeamRelationsLoaded loads team relations if not already loaded.
func (s *NavigationService) ensureTeamRelationsLoaded(ctx context.Context, team *models.Team) error {
	if team.Instance.ID == "" || len(team.CheckIns) == 0 {
		return s.teamRepo.LoadRelations(ctx, team)
	}
	return nil
}

// normalizeMarkerID trims and uppercases marker ID.
func (s *NavigationService) normalizeMarkerID(markerID string) string {
	return strings.TrimSpace(strings.ToUpper(markerID))
}
