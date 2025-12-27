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

func TestLocationService_UpdateCoords(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location with initial coords
	location, err := service.CreateLocation(
		ctx,
		gofakeit.UUID(),
		gofakeit.Name(),
		10.0,
		20.0,
		gofakeit.Number(0, 100))
	require.NoError(t, err)

	// Load marker relation
	err = service.LoadRelations(ctx, &location)
	require.NoError(t, err)

	// Update coords
	newLat := 30.5
	newLng := 40.5
	err = service.UpdateCoords(ctx, &location, newLat, newLng)
	require.NoError(t, err)

	// Verify update
	updatedLocation, err := service.GetByID(ctx, location.ID)
	require.NoError(t, err)
	err = service.LoadRelations(ctx, updatedLocation)
	require.NoError(t, err)

	assert.Equal(t, newLat, updatedLocation.Marker.Lat)
	assert.Equal(t, newLng, updatedLocation.Marker.Lng)
}

func TestLocationService_UpdateName(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location
	location, err := service.CreateLocation(
		ctx,
		gofakeit.UUID(),
		"Original Name",
		gofakeit.Latitude(),
		gofakeit.Longitude(),
		gofakeit.Number(0, 100))
	require.NoError(t, err)

	// Update name
	newName := "Updated Name"
	err = service.UpdateName(ctx, &location, newName)
	require.NoError(t, err)

	// Verify update
	updatedLocation, err := service.GetByID(ctx, location.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, updatedLocation.Name)
}

func TestLocationService_UpdateLocation(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Update all fields", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx,
			gofakeit.UUID(),
			"Original Name",
			10.0,
			20.0,
			50)
		require.NoError(t, err)

		// Update location with new data
		updateData := services.LocationUpdateData{
			Name:      "New Name",
			Latitude:  30.5,
			Longitude: 40.5,
			Points:    100,
		}
		err = service.UpdateLocation(ctx, &location, updateData)
		require.NoError(t, err)

		// Verify update
		updatedLocation, err := service.GetByID(ctx, location.ID)
		require.NoError(t, err)
		err = service.LoadRelations(ctx, updatedLocation)
		require.NoError(t, err)

		assert.Equal(t, updateData.Name, updatedLocation.Name)
		assert.Equal(t, updateData.Latitude, updatedLocation.Marker.Lat)
		assert.Equal(t, updateData.Longitude, updatedLocation.Marker.Lng)
		assert.Equal(t, updateData.Points, updatedLocation.Points)
	})

	t.Run("Invalid latitude", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx,
			gofakeit.UUID(),
			"Test Location",
			10.0,
			20.0,
			50)
		require.NoError(t, err)

		updateData := services.LocationUpdateData{
			Latitude: 100.0, // Invalid: > 90
		}
		err = service.UpdateLocation(ctx, &location, updateData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "latitude must be between -90 and 90")
	})

	t.Run("Invalid longitude", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx,
			gofakeit.UUID(),
			"Test Location",
			10.0,
			20.0,
			50)
		require.NoError(t, err)

		updateData := services.LocationUpdateData{
			Longitude: 200.0, // Invalid: > 180
		}
		err = service.UpdateLocation(ctx, &location, updateData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "longitude must be between -180 and 180")
	})
}

func TestLocationService_ReorderLocations(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Reorder locations successfully", func(t *testing.T) {
		instanceID := gofakeit.UUID()

		// Create 3 locations
		loc1, err := service.CreateLocation(ctx, instanceID, "Location 1", 10.0, 20.0, 10)
		require.NoError(t, err)
		loc2, err := service.CreateLocation(ctx, instanceID, "Location 2", 11.0, 21.0, 20)
		require.NoError(t, err)
		loc3, err := service.CreateLocation(ctx, instanceID, "Location 3", 12.0, 22.0, 30)
		require.NoError(t, err)

		// Reorder: 3, 1, 2
		newOrder := []string{loc3.ID, loc1.ID, loc2.ID}
		err = service.ReorderLocations(ctx, instanceID, newOrder)
		require.NoError(t, err)

		// Verify new order
		locations, err := service.FindByInstance(ctx, instanceID)
		require.NoError(t, err)

		orderMap := make(map[string]int)
		for _, loc := range locations {
			orderMap[loc.ID] = loc.Order
		}

		assert.Equal(t, 0, orderMap[loc3.ID])
		assert.Equal(t, 1, orderMap[loc1.ID])
		assert.Equal(t, 2, orderMap[loc2.ID])
	})

	t.Run("Invalid location ID", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		loc1, err := service.CreateLocation(ctx, instanceID, "Location 1", 10.0, 20.0, 10)
		require.NoError(t, err)

		// Try to reorder with invalid ID
		invalidOrder := []string{loc1.ID, "invalid-id"}
		err = service.ReorderLocations(ctx, instanceID, invalidOrder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid location ID")
	})

	t.Run("Mismatched list length", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		loc1, err := service.CreateLocation(ctx, instanceID, "Location 1", 10.0, 20.0, 10)
		require.NoError(t, err)
		_, err = service.CreateLocation(ctx, instanceID, "Location 2", 11.0, 21.0, 20)
		require.NoError(t, err)

		// Try to reorder with only one ID (should have 2)
		invalidOrder := []string{loc1.ID}
		err = service.ReorderLocations(ctx, instanceID, invalidOrder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list length does not match")
	})
}

func TestLocationService_LoadRelations(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location
	location, err := service.CreateLocation(
		ctx,
		gofakeit.UUID(),
		gofakeit.Name(),
		gofakeit.Latitude(),
		gofakeit.Longitude(),
		gofakeit.Number(0, 100))
	require.NoError(t, err)

	// Initially, marker should not be loaded
	assert.Empty(t, location.Marker.Code)

	// Load relations
	err = service.LoadRelations(ctx, &location)
	require.NoError(t, err)

	// Verify marker is loaded
	assert.NotEmpty(t, location.Marker.Code)
	assert.Equal(t, location.MarkerID, location.Marker.Code)
}

func TestLocationService_LoadBlocks(t *testing.T) {
	service, _, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location (which creates a default header block)
	location, err := service.CreateLocation(
		ctx,
		gofakeit.UUID(),
		gofakeit.Name(),
		gofakeit.Latitude(),
		gofakeit.Longitude(),
		gofakeit.Number(0, 100))
	require.NoError(t, err)

	// Load blocks
	err = service.LoadBlocks(ctx, &location)
	require.NoError(t, err)

	// Verify at least the default header block is loaded
	assert.NotEmpty(t, location.Blocks)
	assert.GreaterOrEqual(t, len(location.Blocks), 1)
}
