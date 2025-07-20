package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupLocationService(t *testing.T) (services.LocationService, *services.MarkerService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	clueRepo := repositories.NewClueRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	markerService := services.NewMarkerService(markerRepo)
	locationService := services.NewLocationService(clueRepo, locationRepo, markerRepo, blockRepo, markerService)
	return locationService, markerService, cleanup
}

func TestLocationService_CreateLocation(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Create location", func(t *testing.T) {
		location, err := service.CreateLocation(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		assert.NoError(t, err)
		assert.NotEmpty(t, location.ID)
	})

	t.Run("Create location with invalid instance ID", func(t *testing.T) {
		_, err := service.CreateLocation(
			context.Background(),
			"",
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		assert.Error(t, err)
	})

	t.Run("Create location with invalid name", func(t *testing.T) {
		_, err := service.CreateLocation(
			context.Background(),
			gofakeit.UUID(),
			"",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		assert.Error(t, err)
	})
}

func TestLocationService_CreateLocationFromMarker(t *testing.T) {
	service, markerService, cleanup := setupLocationService(t)
	defer cleanup()

	marker, err := markerService.CreateMarker(
		context.Background(),
		gofakeit.Name(),
		gofakeit.Latitude(),
		gofakeit.Longitude())
	assert.NoError(t, err)

	t.Run("Create location from marker", func(t *testing.T) {
		location, err := service.CreateLocationFromMarker(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			marker.Code)
		assert.NoError(t, err)
		assert.NotEmpty(t, location.ID)
	})

	t.Run("Create location from marker with invalid instance ID", func(t *testing.T) {
		_, err = service.CreateLocationFromMarker(
			context.Background(),
			"",
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			marker.Code)
		assert.Error(t, err)
	})

	t.Run("Create location from marker with invalid name", func(t *testing.T) {
		_, err = service.CreateLocationFromMarker(
			context.Background(),
			gofakeit.UUID(),
			"",
			gofakeit.Number(0, 100),
			marker.Code)
		assert.Error(t, err)
	})

	t.Run("Create location from marker with invalid marker code", func(t *testing.T) {
		_, err := service.CreateLocationFromMarker(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			"")
		assert.Error(t, err)
	})
}

func TestLocationService_DuplicateLocation(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()

	blockService, blockCleanup := setupBlocksService(t)
	defer blockCleanup()

	t.Run("Duplicate location", func(t *testing.T) {
		// Create a location
		location, err := service.CreateLocation(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		assert.NoError(t, err)

		// Create a block
		_, err = blockService.NewBlock(context.Background(), location.ID, "image")
		assert.NoError(t, err)

		// Duplicate the location
		newLocation, err := service.DuplicateLocation(context.Background(), location, gofakeit.UUID())
		assert.NoError(t, err)
		assert.NotEqual(t, location.ID, newLocation.ID)

		// Check that the location was duplicated
		checkLocation, err := service.GetByID(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.NotNil(t, checkLocation)

		// Check that the blocks were duplicated
		blocks, err := blockService.FindByLocationID(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, blocks, 1)
	})
}

func TestLocationService_GetByID(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Get location by ID", func(t *testing.T) {
		location, err := service.CreateLocation(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		assert.NoError(t, err)

		checkLocation, err := service.GetByID(context.Background(), location.ID)
		assert.NoError(t, err)
		assert.NotNil(t, checkLocation)
	})

	t.Run("Get location by ID with invalid ID", func(t *testing.T) {
		_, err := service.GetByID(context.Background(), "")
		assert.Error(t, err)
	})
}

func TestLocationService_GetByInstanceAndCode(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Get location by instance and code", func(t *testing.T) {
		location, err := service.CreateLocation(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		assert.NoError(t, err)

		checkLocation, err := service.GetByInstanceAndCode(context.Background(), location.InstanceID, location.MarkerID)
		assert.NoError(t, err)
		assert.NotNil(t, checkLocation)
	})

	t.Run("Get location by instance and code with invalid instance ID", func(t *testing.T) {
		_, err := service.GetByInstanceAndCode(context.Background(), "", gofakeit.UUID())
		assert.Error(t, err)
	})
}

func TestLocationService_FindByInstance(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Find locations by instance", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		locations := make([]string, 5)
		for i := 0; i < 5; i++ {
			location, err := service.CreateLocation(
				context.Background(),
				instanceID,
				gofakeit.Name(),
				gofakeit.Latitude(),
				gofakeit.Longitude(),
				gofakeit.Number(0, 100))
			assert.NoError(t, err)
			locations[i] = location.ID
		}

		foundLocations, err := service.FindByInstance(context.Background(), instanceID)
		assert.NoError(t, err)
		assert.Len(t, foundLocations, 5)
	})

	t.Run("Find locations by instance with invalid instance ID", func(t *testing.T) {
		locs, err := service.FindByInstance(context.Background(), "")
		assert.NoError(t, err)
		assert.Empty(t, locs)
	})
}

// TODO: Test the following methods:
// 	// FindMarkersNotInInstance finds all markers that are not in the given instance
// 	FindMarkersNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error)

// TODO: Test the following methods:
// 	// Update visitor stats for a location
// 	IncrementVisitorStats(ctx context.Context, location *models.Location) error
// 	// UpdateCoords updates the coordinates for a location
// 	UpdateCoords(ctx context.Context, location *models.Location, lat, lng float64) error
// 	// UpdateName updates the name of a location
// 	UpdateName(ctx context.Context, location *models.Location, name string) error
// 	// UpdateLocation updates a location
// 	UpdateLocation(ctx context.Context, location *models.Location, data LocationUpdateData) error
// 	// ReorderLocations accepts IDs of locations and reorders them
// 	ReorderLocations(ctx context.Context, instanceID string, locationIDs []string) error
//
// 	// DeleteLocation deletes a location
// 	DeleteLocation(ctx context.Context, locationID string) error
// 	// DeleteByInstanceID deletes all locations for an instance
// 	DeleteLocations(ctx context.Context, tx *bun.Tx, locations []models.Location) error
//
// 	// LoadCluesForLocation loads the clues for a specific location if they are not already loaded
// 	LoadCluesForLocation(ctx context.Context, location *models.Location) error
// 	// LoadCluesForLocations loads the clues for all given locations if they are not already loaded
// 	LoadCluesForLocations(ctx context.Context, locations *[]models.Location) error
// 	// LoadRelations loads the related data for a location
// 	LoadRelations(ctx context.Context, location *models.Location) error
