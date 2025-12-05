package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

type LocationService interface {
	// CreateLocation creates a new location
	CreateLocation(ctx context.Context, instanceID, name string, lat, lng float64, points int) (models.Location, error)
	// CreateLocationFromMarker creates a new location from an existing marker
	CreateLocationFromMarker(
		ctx context.Context,
		instanceID, name string,
		points int,
		markerCode string,
	) (models.Location, error)

	// GetByID finds a location by its ID
	GetByID(ctx context.Context, locationID string) (*models.Location, error)
	// GetByInstanceAndCode finds a location by its instance and code
	GetByInstanceAndCode(ctx context.Context, instanceID string, code string) (*models.Location, error)
	// FindByInstance finds all locations for an instance
	FindByInstance(ctx context.Context, instanceID string) ([]models.Location, error)

	// UpdateCoords updates the coordinates for a location
	UpdateCoords(ctx context.Context, location *models.Location, lat, lng float64) error
	// UpdateName updates the name of a location
	UpdateName(ctx context.Context, location *models.Location, name string) error
	// UpdateLocation updates a location
	UpdateLocation(ctx context.Context, location *models.Location, data LocationUpdateData) error
	// ReorderLocations accepts IDs of locations and reorders them
	ReorderLocations(ctx context.Context, instanceID string, locationIDs []string) error

	// LoadRelations loads the related data for a location
	LoadRelations(ctx context.Context, location *models.Location) error
}

type locationService struct {
	locationRepo  repositories.LocationRepository
	markerRepo    repositories.MarkerRepository
	blockRepo     repositories.BlockRepository
	markerService *MarkerService
}

// NewLocationService creates a new instance of LocationService.
func NewLocationService(
	locationRepo repositories.LocationRepository,
	markerRepo repositories.MarkerRepository,
	blockRepo repositories.BlockRepository,
	markerService *MarkerService,
) LocationService {
	return locationService{
		locationRepo:  locationRepo,
		markerRepo:    markerRepo,
		blockRepo:     blockRepo,
		markerService: markerService,
	}
}

// checkLocationData checks if the provided location data is valid.
func checkLocationData(instanceID, name string, lat, lng float64) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if instanceID == "" {
		return errors.New("instanceID cannot be empty")
	}
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude must be between -90 and 90, got: %f", lat)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("longitude must be between -180 and 180, got: %f", lng)
	}
	return nil
}

// CreateLocation creates a new location.
func (s locationService) CreateLocation(
	ctx context.Context,
	instanceID, name string,
	lat, lng float64,
	points int,
) (models.Location, error) {
	if err := checkLocationData(instanceID, name, lat, lng); err != nil {
		return models.Location{}, err
	}

	// Create the marker
	marker, err := s.markerService.CreateMarker(ctx, name, lat, lng)
	if err != nil {
		return models.Location{}, fmt.Errorf("creating marker: %w", err)
	}

	location := models.Location{
		Name:       name,
		InstanceID: instanceID,
		MarkerID:   marker.Code,
		Points:     points,
	}
	err = s.locationRepo.Create(ctx, &location)
	if err != nil {
		return models.Location{}, fmt.Errorf("saving location: %w", err)
	}

	// Every location gets a default header block
	if err := s.createDefaultHeaderBlock(ctx, &location); err != nil {
		return models.Location{}, fmt.Errorf("creating default header block: %w", err)
	}

	return location, nil
}

// CreateLocationFromMarker creates a new location from an existing marker.
func (s locationService) CreateLocationFromMarker(
	ctx context.Context,
	instanceID, name string,
	points int,
	markerCode string,
) (models.Location, error) {
	if err := checkLocationData(instanceID, name, 0, 0); err != nil {
		return models.Location{}, err
	}

	marker, err := s.markerRepo.GetByCode(ctx, markerCode)
	if err != nil {
		return models.Location{}, fmt.Errorf("finding marker: %w", err)
	}

	location := models.Location{
		Name:       name,
		InstanceID: instanceID,
		MarkerID:   marker.Code,
		Points:     points,
	}
	err = s.locationRepo.Create(ctx, &location)
	if err != nil {
		return models.Location{}, fmt.Errorf("saving location: %w", err)
	}

	// Every location gets a default header block
	if err := s.createDefaultHeaderBlock(ctx, &location); err != nil {
		return models.Location{}, fmt.Errorf("creating default header block: %w", err)
	}

	return location, nil
}

// createDefaultHeaderBlock creates a default header block for a newly created location.
func (s locationService) createDefaultHeaderBlock(ctx context.Context, location *models.Location) error {
	blockID := uuid.New().String()
	headerData := map[string]string{
		"icon":       "map-pin-check-inside",
		"title_text": location.Name,
		"title_size": "large",
	}

	jsonData, err := json.Marshal(headerData)
	if err != nil {
		return fmt.Errorf("marshaling header block data: %w", err)
	}

	baseBlock := blocks.BaseBlock{
		ID:         blockID,
		LocationID: location.ID,
		Type:       "header",
		Data:       jsonData,
		Order:      0,
		Points:     0,
	}

	block, err := blocks.CreateFromBaseBlock(baseBlock)
	if err != nil {
		return fmt.Errorf("creating header block instance: %w", err)
	}

	err = block.ParseData()
	if err != nil {
		return fmt.Errorf("parsing header block data: %w", err)
	}

	_, err = s.blockRepo.Create(ctx, block, location.ID, blocks.ContextLocationContent)
	if err != nil {
		return fmt.Errorf("saving header block: %w", err)
	}

	return nil
}

// GetByID finds a location by ID.
func (s locationService) GetByID(ctx context.Context, locationID string) (*models.Location, error) {
	location, err := s.locationRepo.GetByID(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("finding location: %w", err)
	}
	return location, nil
}

// GetByInstanceAndCode finds a location by instance and code.
func (s locationService) GetByInstanceAndCode(
	ctx context.Context,
	instanceID string,
	code string,
) (*models.Location, error) {
	location, err := s.locationRepo.GetByInstanceAndCode(ctx, instanceID, code)
	if err != nil {
		return nil, fmt.Errorf("finding location by instance and code: %w", err)
	}
	return location, nil
}

// FindByInstance finds all locations for an instance.
func (s locationService) FindByInstance(ctx context.Context, instanceID string) ([]models.Location, error) {
	locations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("finding all locations: %w", err)
	}
	return locations, nil
}

// UpdateCoords updates the coordinates for a location.
func (s locationService) UpdateCoords(ctx context.Context, location *models.Location, lat, lng float64) error {
	location.Marker.Lat = lat
	location.Marker.Lng = lng
	return s.markerRepo.Update(ctx, &location.Marker)
}

// UpdateName updates the name of a location.
func (s locationService) UpdateName(ctx context.Context, location *models.Location, name string) error {
	location.Name = name
	return s.locationRepo.Update(ctx, location)
}

func (s locationService) UpdateLocation(ctx context.Context, location *models.Location, data LocationUpdateData) error {
	if location.Marker.Code == "" {
		err := s.locationRepo.LoadMarker(ctx, location)
		if err != nil {
			return fmt.Errorf("loading marker: %w", err)
		}
	}

	// Set up the marker data
	update := false

	if data.Name != "" && data.Name != location.Marker.Name {
		location.Marker.Name = data.Name
		update = true
	}

	if data.Latitude >= -90 && data.Latitude <= 90 && data.Latitude != location.Marker.Lat {
		location.Marker.Lat = data.Latitude
		update = true
	} else if data.Latitude < -90 || data.Latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90, got: %f", data.Latitude)
	}

	if data.Longitude >= -180 && data.Longitude <= 180 && data.Longitude != location.Marker.Lng {
		location.Marker.Lng = data.Longitude
		update = true
	} else if data.Longitude < -180 || data.Longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180, got: %f", data.Longitude)
	}

	// To avoid updating markers that other games are using, we need to check if the marker is shared
	shared, err := s.markerRepo.IsShared(ctx, location.Marker.Code)
	if err != nil {
		return fmt.Errorf("checking if marker is shared: %w", err)
	}

	if shared && update {
		newMarker, createErr := s.markerService.CreateMarker(
			ctx,
			location.Marker.Name,
			location.Marker.Lat,
			location.Marker.Lng,
		)
		if createErr != nil {
			return fmt.Errorf("creating new marker: %w", createErr)
		}
		location.MarkerID = newMarker.Code
	} else if update {
		if updateErr := s.markerRepo.Update(ctx, &location.Marker); updateErr != nil {
			return fmt.Errorf("updating marker: %w", updateErr)
		}
	}

	// Now that the marker is updated, we can update the location
	// We'll assume if the marker was updated a new one was created

	if data.Points >= 0 && data.Points != location.Points {
		location.Points = data.Points
		update = true
	}

	if data.Name != "" && data.Name != location.Name {
		location.Name = data.Name
		update = true
	}

	if update {
		if updateErr := s.locationRepo.Update(ctx, location); updateErr != nil {
			return fmt.Errorf("updating location: %w", updateErr)
		}
	}

	return nil
}

// ReorderLocations reorders locations.
func (s locationService) ReorderLocations(ctx context.Context, instanceID string, locationIDs []string) error {
	locations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("finding all locations: %w", err)
	}

	// Check that all location IDs are valid
	locationMap := make(map[string]bool)
	for _, location := range locations {
		locationMap[location.ID] = true
	}
	for _, locationID := range locationIDs {
		if !locationMap[locationID] {
			return fmt.Errorf("invalid location ID: %s", locationID)
		}
	}
	if len(locationMap) != len(locationIDs) {
		return errors.New("list length does not match number of locations")
	}

	// Reorder the locations
	for i, locationID := range locationIDs {
		for j, location := range locations {
			if location.ID == locationID {
				locations[j].Order = i
				break
			}
		}
	}

	// Save the locations
	for _, location := range locations {
		err = s.locationRepo.Update(ctx, &location)
		if err != nil {
			return fmt.Errorf("updating location: %w", err)
		}
	}

	return nil
}

// LoadRelations loads the related data for a location.
func (s locationService) LoadRelations(ctx context.Context, location *models.Location) error {
	err := s.locationRepo.LoadRelations(ctx, location)
	if err != nil {
		return fmt.Errorf("loading relations: %w", err)
	}
	return nil
}
