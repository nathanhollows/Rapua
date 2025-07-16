package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type gameManagerService struct {
	locationService LocationService
	markerRepo      repositories.MarkerRepository
	instanceService InstanceService
}

// TODO: Split this service into smaller services.
type GameManagerService interface {
	CreateLocation(ctx context.Context, user *models.User, data map[string]string) (models.Location, error)
	SaveLocation(ctx context.Context, location *models.Location, lat, lng, name string) error

	// Marker & Validation
	ValidateLocationMarker(user *models.User, id string) bool
	ValidateLocationID(user *models.User, id string) bool
}

func NewGameManagerService(
	locationService LocationService,
	markerRepo repositories.MarkerRepository,
	instanceService InstanceService,
) GameManagerService {
	return &gameManagerService{
		locationService: locationService,
		markerRepo:      markerRepo,
		instanceService: instanceService,
	}
}

func (s *gameManagerService) SaveLocation(ctx context.Context, location *models.Location, lat, lng, name string) error {
	if lat == "" || lng == "" {
		return errors.New("latitude and longitude are required")
	}

	latFloat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		return err
	}
	lngFloat, err := strconv.ParseFloat(lng, 64)
	if err != nil {
		return err
	}

	err = s.locationService.UpdateCoords(ctx, location, latFloat, lngFloat)
	if err != nil {
		return fmt.Errorf("updating location coordinates: %w", err)
	}

	err = s.locationService.UpdateName(ctx, location, name)
	if err != nil {
		return fmt.Errorf("updating location name: %w", err)
	}

	return nil
}

func (s *gameManagerService) CreateLocation(
	ctx context.Context,
	user *models.User,
	data map[string]string,
) (models.Location, error) {
	// Extract input values
	name := data["name"]
	latStr := data["latitude"]
	lngStr := data["longitude"]
	pointsStr := data["points"]
	markerCode := data["marker"]

	var (
		lat float64
		lng float64
		err error
	)

	// Parse latitude / longitude if provided
	if latStr != "" && lngStr != "" {
		lat, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return models.Location{}, fmt.Errorf("invalid latitude: %w", err)
		}
		lng, err = strconv.ParseFloat(lngStr, 64)
		if err != nil {
			return models.Location{}, fmt.Errorf("invalid longitude: %w", err)
		}
	}

	// Parse points (default = 10)
	points := 10
	if pointsStr != "" {
		points, err = strconv.Atoi(pointsStr)
		if err != nil {
			return models.Location{}, fmt.Errorf("invalid points value: %w", err)
		}
	}

	// If no marker code given, create a location directly
	if markerCode == "" {
		return s.locationService.CreateLocation(
			ctx,
			user.CurrentInstanceID,
			name,
			lat,
			lng,
			points,
		)
	}

	// Otherwise, verify that marker code exists in markers not already in the user’s current instance
	instanceIDs, err := s.instanceService.FindInstanceIDsForUser(ctx, user.ID)
	if err != nil {
		return models.Location{}, fmt.Errorf("getting instance IDs for user: %w", err)
	}

	markers, err := s.markerRepo.FindNotInInstance(ctx, user.CurrentInstanceID, instanceIDs)
	if err != nil {
		return models.Location{}, fmt.Errorf("finding markers not in instance: %w", err)
	}

	// Check if the requested marker code exists among returned markers
	markerExists := false
	for _, m := range markers {
		if m.Code == markerCode {
			markerExists = true
			break
		}
	}
	if !markerExists {
		return models.Location{}, errors.New("marker does not exist")
	}

	// Finally, create location from marker
	return s.locationService.CreateLocationFromMarker(
		ctx,
		user.CurrentInstanceID,
		name,
		points,
		markerCode,
	)
}

func (s *gameManagerService) ValidateLocationMarker(user *models.User, id string) bool {
	for _, loc := range user.CurrentInstance.Locations {
		if loc.MarkerID == id {
			return true
		}
	}
	return false
}

func (s *gameManagerService) ValidateLocationID(user *models.User, id string) bool {
	for _, loc := range user.CurrentInstance.Locations {
		if loc.ID == id {
			return true
		}
	}
	return false
}
