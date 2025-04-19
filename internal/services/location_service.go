package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/uptrace/bun"
)

type LocationService interface {
	// CreateLocation creates a new location
	CreateLocation(ctx context.Context, instanceID, name string, lat, lng float64, points int) (models.Location, error)
	// CreateLocationFromMarker creates a new location from an existing marker
	CreateLocationFromMarker(ctx context.Context, instanceID, name string, points int, markerCode string) (models.Location, error)
	// CreateMarker creates a new marker
	CreateMarker(ctx context.Context, name string, lat, lng float64) (models.Marker, error)
	// DuplicateLocation creates a new location given an existing location and the instance ID of the new location
	DuplicateLocation(ctx context.Context, location models.Location, newInstanceID string) (models.Location, error)

	// GetByID finds a location by its ID
	GetByID(ctx context.Context, locationID string) (*models.Location, error)
	// GetByInstanceAndCode finds a location by its instance and code
	GetByInstanceAndCode(ctx context.Context, instanceID string, code string) (*models.Location, error)
	// FindByInstance finds all locations for an instance
	FindByInstance(ctx context.Context, instanceID string) ([]models.Location, error)
	// FindMarkersNotInInstance finds all markers that are not in the given instance
	FindMarkersNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error)

	// Update visitor stats for a location
	IncrementVisitorStats(ctx context.Context, location *models.Location) error
	// UpdateCoords updates the coordinates for a location
	UpdateCoords(ctx context.Context, location *models.Location, lat, lng float64) error
	// UpdateName updates the name of a location
	UpdateName(ctx context.Context, location *models.Location, name string) error
	// UpdateLocation updates a location
	UpdateLocation(ctx context.Context, location *models.Location, data LocationUpdateData) error
	// ReorderLocations accepts IDs of locations and reorders them
	ReorderLocations(ctx context.Context, instanceID string, locationIDs []string) error

	// DeleteLocation deletes a location
	DeleteLocation(ctx context.Context, locationID string) error
	// DeleteByInstanceID deletes all locations for an instance
	DeleteLocations(ctx context.Context, tx *bun.Tx, locations []models.Location) error

	// LoadCluesForLocation loads the clues for a specific location if they are not already loaded
	LoadCluesForLocation(ctx context.Context, location *models.Location) error
	// LoadCluesForLocations loads the clues for all given locations if they are not already loaded
	LoadCluesForLocations(ctx context.Context, locations *[]models.Location) error
	// LoadRelations loads the related data for a location
	LoadRelations(ctx context.Context, location *models.Location) error
}

type locationService struct {
	transactor   db.Transactor
	locationRepo repositories.LocationRepository
	clueRepo     repositories.ClueRepository
	markerRepo   repositories.MarkerRepository
	blockRepo    repositories.BlockRepository
}

// NewLocationService creates a new instance of LocationService.
func NewLocationService(
	transactor db.Transactor,
	clueRepo repositories.ClueRepository,
	locationRepo repositories.LocationRepository,
	markerRepo repositories.MarkerRepository,
	blockRepo repositories.BlockRepository,
) LocationService {
	return locationService{
		transactor:   transactor,
		clueRepo:     clueRepo,
		locationRepo: locationRepo,
		markerRepo:   markerRepo,
		blockRepo:    blockRepo,
	}
}

// CreateLocation creates a new location.
func (s locationService) CreateLocation(ctx context.Context, instanceID, name string, lat, lng float64, points int) (models.Location, error) {
	if name == "" {
		return models.Location{}, NewValidationError("name")
	}
	if instanceID == "" {
		return models.Location{}, NewValidationError("instanceID")
	}

	// Create the marker
	marker, err := s.CreateMarker(ctx, name, lat, lng)
	if err != nil {
		return models.Location{}, fmt.Errorf("creating marker: %v", err)
	}

	location := models.Location{
		Name:       name,
		InstanceID: instanceID,
		MarkerID:   marker.Code,
		Points:     points,
	}
	err = s.locationRepo.Create(ctx, &location)
	if err != nil {
		return models.Location{}, fmt.Errorf("saving location: %v", err)
	}

	return location, nil
}

// CreateLocationFromMarker creates a new location from an existing marker.
func (s locationService) CreateLocationFromMarker(ctx context.Context, instanceID, name string, points int, markerCode string) (models.Location, error) {
	if name == "" {
		return models.Location{}, NewValidationError("name")
	}
	if instanceID == "" {
		return models.Location{}, NewValidationError("instanceID")
	}
	marker, err := s.markerRepo.GetByCode(ctx, markerCode)
	if err != nil {
		return models.Location{}, fmt.Errorf("finding marker: %v", err)
	}

	location := models.Location{
		Name:       name,
		InstanceID: instanceID,
		MarkerID:   marker.Code,
		Points:     points,
	}
	err = s.locationRepo.Create(ctx, &location)
	if err != nil {
		return models.Location{}, fmt.Errorf("saving location: %v", err)
	}

	return location, nil
}

// CreateMarker creates a new marker.
func (s locationService) CreateMarker(ctx context.Context, name string, lat, lng float64) (models.Marker, error) {
	marker := models.Marker{
		Name: name,
		Lat:  lat,
		Lng:  lng,
	}
	err := s.markerRepo.Create(ctx, &marker)
	if err != nil {
		return models.Marker{}, fmt.Errorf("saving marker: %v", err)
	}
	return marker, nil
}

// DuplicateLocation duplicates a location.
func (s locationService) DuplicateLocation(ctx context.Context, location models.Location, newInstanceID string) (models.Location, error) {
	// Load relations
	err := s.locationRepo.LoadRelations(ctx, &location)
	if err != nil {
		return models.Location{}, fmt.Errorf("loading relations: %v", err)
	}

	// Copy the location
	newLocation := location
	newLocation.ID = ""
	newLocation.InstanceID = newInstanceID
	err = s.locationRepo.Create(ctx, &newLocation)
	if err != nil {
		return models.Location{}, fmt.Errorf("saving location: %v", err)
	}

	// Copy the clues
	for _, clue := range location.Clues {
		newClue := clue
		newClue.ID = ""
		newClue.InstanceID = newInstanceID
		newClue.LocationID = newLocation.ID
		err = s.clueRepo.Save(ctx, &newClue)
		if err != nil {
			return models.Location{}, fmt.Errorf("saving clue: %v", err)
		}
	}

	// Copy the blocks
	for _, block := range location.Blocks {
		block, err := s.blockRepo.GetByID(ctx, block.ID)
		if err != nil {
			return models.Location{}, fmt.Errorf("finding block: %v", err)
		}
		_, err = s.blockRepo.Create(ctx, block, newLocation.ID)
		if err != nil {
			return models.Location{}, fmt.Errorf("saving block: %v", err)
		}
	}

	return newLocation, nil
}

// GetByID finds a location by ID.
func (s locationService) GetByID(ctx context.Context, locationID string) (*models.Location, error) {
	location, err := s.locationRepo.GetByID(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("finding location: %v", err)
	}
	return location, nil
}

// GetByInstanceAndCode finds a location by instance and code.
func (s locationService) GetByInstanceAndCode(ctx context.Context, instanceID string, code string) (*models.Location, error) {
	location, err := s.locationRepo.GetByInstanceAndCode(ctx, instanceID, code)
	if err != nil {
		return nil, fmt.Errorf("finding location by instance and code: %v", err)
	}
	return location, nil
}

// FindByInstance finds all locations for an instance.
func (s locationService) FindByInstance(ctx context.Context, instanceID string) ([]models.Location, error) {
	locations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("finding all locations: %v", err)
	}
	return locations, nil
}

// FindMarkersNotInInstance finds all markers that are not in the given instance.
func (s locationService) FindMarkersNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error) {
	markers, err := s.markerRepo.FindNotInInstance(ctx, instanceID, otherInstances)
	if err != nil {
		return nil, fmt.Errorf("finding markers not in instance: %v", err)
	}
	return markers, nil
}

// Update visitor stats for a location.
func (s locationService) IncrementVisitorStats(ctx context.Context, location *models.Location) error {
	location.CurrentCount++
	location.TotalVisits++
	return s.locationRepo.Update(ctx, location)
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
			return fmt.Errorf("loading marker: %v", err)
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
	}

	if data.Longitude >= -180 && data.Longitude <= 180 && data.Longitude != location.Marker.Lng {
		location.Marker.Lng = data.Longitude
		update = true
	}

	// To avoid updating markers that other games are using, we need to check if the marker is shared
	shared, err := s.markerRepo.IsShared(ctx, location.Marker.Code)
	if err != nil {
		return fmt.Errorf("checking if marker is shared: %v", err)
	}

	if shared && update {
		newMarker, err := s.CreateMarker(ctx, location.Marker.Name, location.Marker.Lat, location.Marker.Lng)
		if err != nil {
			return fmt.Errorf("creating new marker: %v", err)
		}
		location.MarkerID = newMarker.Code
	} else if update {
		err := s.markerRepo.Update(ctx, &location.Marker)
		if err != nil {
			return fmt.Errorf("updating marker: %v", err)
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
		err := s.locationRepo.Update(ctx, location)
		if err != nil {
			return fmt.Errorf("updating location: %v", err)
		}
	}

	return nil
}

// ReorderLocations reorders locations.
func (s locationService) ReorderLocations(ctx context.Context, instanceID string, locationIDs []string) error {
	locations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("finding all locations: %v", err)
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
			return fmt.Errorf("updating location: %v", err)
		}
	}

	return nil
}

// If the marker is not used by any other locations, it is also deleted.
func (s locationService) DeleteLocation(ctx context.Context, locationID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %v", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				panic(fmt.Errorf("rolling back transaction: %v; %v", p, rollbackErr))
			}
			panic(p)
		}
	}()

	err = s.deleteLocation(ctx, tx, locationID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %v; %v", err, rollbackErr)
		}
		return fmt.Errorf("deleting location: %v", err)
	}

	err = s.markerRepo.DeleteUnused(ctx, tx)
	if err != nil {
		return fmt.Errorf("deleting unused markers: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %v; %v", err, rollbackErr)
		}
		return fmt.Errorf("committing transaction: %v", err)
	}

	return nil
}

// DeleteByInstanceID deletes all locations for an instance.
func (s locationService) DeleteLocations(ctx context.Context, tx *bun.Tx, locations []models.Location) error {
	for _, location := range locations {
		err := s.deleteLocation(ctx, tx, location.ID)
		if err != nil {
			return fmt.Errorf("deleting location: %v", err)
		}
	}

	err := s.markerRepo.DeleteUnused(ctx, tx)
	if err != nil {
		return fmt.Errorf("deleting unused markers: %v", err)
	}

	return nil
}

// deleteLocation deletes a location and its related data.
func (s locationService) deleteLocation(ctx context.Context, tx *bun.Tx, locationID string) error {
	// Delete all related clues
	err := s.clueRepo.DeleteByLocationIDWithTransaction(ctx, tx, locationID)
	if err != nil {
		return fmt.Errorf("deleting clues: %v", err)
	}

	// Delete all related blocks
	err = s.blockRepo.DeleteByLocationID(ctx, tx, locationID)
	if err != nil {
		return fmt.Errorf("deleting blocks: %v", err)
	}

	// Delete the location
	err = s.locationRepo.Delete(ctx, tx, locationID)
	if err != nil {
		return fmt.Errorf("deleting location: %v", err)
	}

	return nil
}

// LoadCluesForLocation loads the clues for a specific location if they are not already loaded.
func (s locationService) LoadCluesForLocation(ctx context.Context, location *models.Location) error {
	if len(location.Clues) == 0 {
		clues, err := s.clueRepo.FindCluesByLocation(ctx, location.ID)
		if err != nil {
			return fmt.Errorf("finding clues: %v", err)
		}
		location.Clues = clues
	}
	return nil
}

// LoadCluesForLocations loads the clues for all given locations if they are not already loaded.
func (s locationService) LoadCluesForLocations(ctx context.Context, locations *[]models.Location) error {
	for i := range *locations {
		err := s.LoadCluesForLocation(ctx, &(*locations)[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadRelations loads the related data for a location.
func (s locationService) LoadRelations(ctx context.Context, location *models.Location) error {
	err := s.locationRepo.LoadRelations(ctx, location)
	if err != nil {
		return fmt.Errorf("loading relations: %v", err)
	}
	return nil
}
