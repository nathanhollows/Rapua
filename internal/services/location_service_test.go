package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLocationService(t *testing.T) (services.LocationService, *services.MarkerService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	markerService := services.NewMarkerService(markerRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
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
		require.NoError(t, err)
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
		require.Error(t, err)
	})

	t.Run("Create location with invalid name", func(t *testing.T) {
		_, err := service.CreateLocation(
			context.Background(),
			gofakeit.UUID(),
			"",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		require.Error(t, err)
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
	require.NoError(t, err)

	t.Run("Create location from marker", func(t *testing.T) {
		location, createErr := service.CreateLocationFromMarker(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			marker.Code)
		require.NoError(t, createErr)
		assert.NotEmpty(t, location.ID)
	})

	t.Run("Create location from marker with invalid instance ID", func(t *testing.T) {
		_, err = service.CreateLocationFromMarker(
			context.Background(),
			"",
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			marker.Code)
		require.Error(t, err)
	})

	t.Run("Create location from marker with invalid name", func(t *testing.T) {
		_, err = service.CreateLocationFromMarker(
			context.Background(),
			gofakeit.UUID(),
			"",
			gofakeit.Number(0, 100),
			marker.Code)
		require.Error(t, err)
	})

	t.Run("Create location from marker with invalid marker code", func(t *testing.T) {
		_, createErr := service.CreateLocationFromMarker(
			context.Background(),
			gofakeit.UUID(),
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			"")
		require.Error(t, createErr)
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
		require.NoError(t, err)

		checkLocation, err := service.GetByID(context.Background(), location.ID)
		require.NoError(t, err)
		assert.NotNil(t, checkLocation)
	})

	t.Run("Get location by ID with invalid ID", func(t *testing.T) {
		_, err := service.GetByID(context.Background(), "")
		require.Error(t, err)
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
		require.NoError(t, err)

		checkLocation, err := service.GetByInstanceAndCode(context.Background(), location.InstanceID, location.MarkerID)
		require.NoError(t, err)
		assert.NotNil(t, checkLocation)
	})

	t.Run("Get location by instance and code with invalid instance ID", func(t *testing.T) {
		_, err := service.GetByInstanceAndCode(context.Background(), "", gofakeit.UUID())
		require.Error(t, err)
	})
}

func TestLocationService_FindByInstance(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Find locations by instance", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		locations := make([]string, 5)
		for i := range 5 {
			location, err := service.CreateLocation(
				context.Background(),
				instanceID,
				gofakeit.Name(),
				gofakeit.Latitude(),
				gofakeit.Longitude(),
				gofakeit.Number(0, 100))
			require.NoError(t, err)
			locations[i] = location.ID
		}

		foundLocations, err := service.FindByInstance(context.Background(), instanceID)
		require.NoError(t, err)
		assert.Len(t, foundLocations, 5)
	})

	t.Run("Find locations by instance with invalid instance ID", func(t *testing.T) {
		locs, err := service.FindByInstance(context.Background(), "")
		require.NoError(t, err)
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
// 	// LoadRelations loads the related data for a location
// 	LoadRelations(ctx context.Context, location *models.Location) error
