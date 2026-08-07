package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupLocationService(t *testing.T) (services.LocationService, *services.MarkerService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	markerService := services.NewMarkerService(markerRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
	return locationService, markerService, dbc, cleanup
}

// validQuestID creates a FK-valid user+quest and returns the quest ID.
func validQuestID(t *testing.T, dbc *bun.DB) string {
	t.Helper()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	return insertTestInstance(t, dbc, userID)
}

func TestLocationService_CreateLocation(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Create location", func(t *testing.T) {
		location, err := service.CreateLocation(
			context.Background(),
			validQuestID(t, dbc),
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
			validQuestID(t, dbc),
			"",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		require.Error(t, err)
	})
}

func TestLocationService_CreateLocationFromMarker(t *testing.T) {
	service, markerService, dbc, cleanup := setupLocationService(t)
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
			validQuestID(t, dbc),
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
			validQuestID(t, dbc),
			"",
			gofakeit.Number(0, 100),
			marker.Code)
		require.Error(t, err)
	})

	t.Run("Create location from marker with invalid marker code", func(t *testing.T) {
		_, createErr := service.CreateLocationFromMarker(
			context.Background(),
			validQuestID(t, dbc),
			gofakeit.Name(),
			gofakeit.Number(0, 100),
			"")
		require.Error(t, createErr)
	})
}

func TestLocationService_GetByID(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Get location by ID", func(t *testing.T) {
		location, err := service.CreateLocation(
			context.Background(),
			validQuestID(t, dbc),
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
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Get location by instance and code", func(t *testing.T) {
		location, err := service.CreateLocation(
			context.Background(),
			validQuestID(t, dbc),
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			gofakeit.Number(0, 100))
		require.NoError(t, err)

		checkLocation, err := service.GetByInstanceAndCode(context.Background(), location.QuestID, location.MarkerID)
		require.NoError(t, err)
		assert.NotNil(t, checkLocation)
	})

	t.Run("Get location by instance and code with invalid instance ID", func(t *testing.T) {
		_, err := service.GetByInstanceAndCode(context.Background(), "", gofakeit.UUID())
		require.Error(t, err)
	})
}

func TestLocationService_FindByInstance(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()

	t.Run("Find locations by instance", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		locations := make([]string, 5)
		for i := range 5 {
			location, err := service.CreateLocation(
				context.Background(),
				questID,
				gofakeit.Name(),
				gofakeit.Latitude(),
				gofakeit.Longitude(),
				gofakeit.Number(0, 100))
			require.NoError(t, err)
			locations[i] = location.ID
		}

		foundLocations, err := service.FindByInstance(context.Background(), questID)
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
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location with initial coords
	location, err := service.CreateLocation(
		ctx,
		validQuestID(t, dbc),
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

	assert.InDelta(t, newLat, updatedLocation.Marker.Lat, 0.0001)
	assert.InDelta(t, newLng, updatedLocation.Marker.Lng, 0.0001)
}

func TestLocationService_UpdateName(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location
	location, err := service.CreateLocation(
		ctx,
		validQuestID(t, dbc),
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
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Update all fields", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx,
			validQuestID(t, dbc),
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
		assert.InDelta(t, updateData.Latitude, updatedLocation.Marker.Lat, 0.0001)
		assert.InDelta(t, updateData.Longitude, updatedLocation.Marker.Lng, 0.0001)
		assert.Equal(t, updateData.Points, updatedLocation.Points)
	})

	t.Run("Invalid latitude", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx,
			validQuestID(t, dbc),
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
			validQuestID(t, dbc),
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
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Reorder locations successfully", func(t *testing.T) {
		questID := validQuestID(t, dbc)

		// Create 3 locations
		loc1, err := service.CreateLocation(ctx, questID, "Location 1", 10.0, 20.0, 10)
		require.NoError(t, err)
		loc2, err := service.CreateLocation(ctx, questID, "Location 2", 11.0, 21.0, 20)
		require.NoError(t, err)
		loc3, err := service.CreateLocation(ctx, questID, "Location 3", 12.0, 22.0, 30)
		require.NoError(t, err)

		// Reorder: 3, 1, 2
		newOrder := []string{loc3.ID, loc1.ID, loc2.ID}
		err = service.ReorderLocations(ctx, questID, newOrder)
		require.NoError(t, err)

		// Verify new order
		locations, err := service.FindByInstance(ctx, questID)
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
		questID := validQuestID(t, dbc)
		loc1, err := service.CreateLocation(ctx, questID, "Location 1", 10.0, 20.0, 10)
		require.NoError(t, err)

		// Try to reorder with invalid ID
		invalidOrder := []string{loc1.ID, "invalid-id"}
		err = service.ReorderLocations(ctx, questID, invalidOrder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid location ID")
	})

	t.Run("Mismatched list length", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		loc1, err := service.CreateLocation(ctx, questID, "Location 1", 10.0, 20.0, 10)
		require.NoError(t, err)
		_, err = service.CreateLocation(ctx, questID, "Location 2", 11.0, 21.0, 20)
		require.NoError(t, err)

		// Try to reorder with only one ID (should have 2)
		invalidOrder := []string{loc1.ID}
		err = service.ReorderLocations(ctx, questID, invalidOrder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list length does not match")
	})
}

func TestLocationService_LoadRelations(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location
	location, err := service.CreateLocation(
		ctx,
		validQuestID(t, dbc),
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

func TestLocationService_GetByInstanceAndSlug(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	questID := validQuestID(t, dbc)
	location, err := service.CreateLocation(
		ctx,
		questID,
		gofakeit.Name(),
		gofakeit.Latitude(),
		gofakeit.Longitude(),
		0,
	)
	require.NoError(t, err)
	require.NotEmpty(t, location.Slug)

	t.Run("found by slug", func(t *testing.T) {
		found, findErr := service.GetByInstanceAndSlug(ctx, questID, location.Slug)
		require.NoError(t, findErr)
		assert.Equal(t, location.ID, found.ID)
	})

	t.Run("not found with wrong slug", func(t *testing.T) {
		_, findErr := service.GetByInstanceAndSlug(ctx, questID, gofakeit.Word())
		require.Error(t, findErr)
	})

	t.Run("not found in wrong instance", func(t *testing.T) {
		_, findErr := service.GetByInstanceAndSlug(ctx, gofakeit.UUID(), location.Slug)
		require.Error(t, findErr)
	})
}

func TestLocationService_CreateLocation_Slug(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug generated from name", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx, validQuestID(t, dbc), "Citrus Collection",
			gofakeit.Latitude(), gofakeit.Longitude(), 0,
		)
		require.NoError(t, err)
		assert.Equal(t, "citrus-collection", location.Slug)
	})

	t.Run("duplicate name in same instance gets unique slug", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		loc1, err := service.CreateLocation(
			ctx,
			questID,
			"Garden Walk",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			0,
		)
		require.NoError(t, err)
		loc2, err := service.CreateLocation(
			ctx,
			questID,
			"Garden Walk",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			0,
		)
		require.NoError(t, err)
		assert.NotEqual(t, loc1.Slug, loc2.Slug)
	})

	t.Run("same name in different instances can share slug", func(t *testing.T) {
		loc1, err := service.CreateLocation(
			ctx,
			validQuestID(t, dbc),
			"Garden Walk",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			0,
		)
		require.NoError(t, err)
		loc2, err := service.CreateLocation(
			ctx,
			validQuestID(t, dbc),
			"Garden Walk",
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			0,
		)
		require.NoError(t, err)
		assert.Equal(t, loc1.Slug, loc2.Slug)
	})
}

func TestLocationService_CreateLocationFromMarker_Slug(t *testing.T) {
	service, markerService, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug generated from name", func(t *testing.T) {
		marker, err := markerService.CreateMarker(ctx, gofakeit.Name(), gofakeit.Latitude(), gofakeit.Longitude())
		require.NoError(t, err)
		location, err := service.CreateLocationFromMarker(ctx, validQuestID(t, dbc), "Rose Garden", 0, marker.Code)
		require.NoError(t, err)
		assert.Equal(t, "rose-garden", location.Slug)
	})

	t.Run("duplicate name in same instance gets unique slug", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		marker1, err := markerService.CreateMarker(ctx, gofakeit.Name(), gofakeit.Latitude(), gofakeit.Longitude())
		require.NoError(t, err)
		marker2, err := markerService.CreateMarker(ctx, gofakeit.Name(), gofakeit.Latitude(), gofakeit.Longitude())
		require.NoError(t, err)
		loc1, err := service.CreateLocationFromMarker(ctx, questID, "Rose Garden", 0, marker1.Code)
		require.NoError(t, err)
		loc2, err := service.CreateLocationFromMarker(ctx, questID, "Rose Garden", 0, marker2.Code)
		require.NoError(t, err)
		assert.NotEqual(t, loc1.Slug, loc2.Slug)
	})
}

func TestLocationService_UpdateLocation_Slug(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug updates when name changes", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx, validQuestID(t, dbc), "Original Name",
			gofakeit.Latitude(), gofakeit.Longitude(), 0,
		)
		require.NoError(t, err)
		assert.Equal(t, "original-name", location.Slug)

		err = service.UpdateLocation(ctx, &location, services.LocationUpdateData{Name: "New Name"})
		require.NoError(t, err)
		assert.Equal(t, "new-name", location.Slug)
	})

	t.Run("slug unchanged when name not provided", func(t *testing.T) {
		location, err := service.CreateLocation(
			ctx, validQuestID(t, dbc), "Original Name",
			gofakeit.Latitude(), gofakeit.Longitude(), 0,
		)
		require.NoError(t, err)
		originalSlug := location.Slug

		err = service.UpdateLocation(ctx, &location, services.LocationUpdateData{Points: 10})
		require.NoError(t, err)
		assert.Equal(t, originalSlug, location.Slug)
	})

	t.Run("duplicate name in same instance gets unique slug on rename", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		_, err := service.CreateLocation(ctx, questID, "Garden Walk", gofakeit.Latitude(), gofakeit.Longitude(), 0)
		require.NoError(t, err)
		loc2, err := service.CreateLocation(
			ctx,
			questID,
			gofakeit.Name(),
			gofakeit.Latitude(),
			gofakeit.Longitude(),
			0,
		)
		require.NoError(t, err)

		err = service.UpdateLocation(ctx, &loc2, services.LocationUpdateData{Name: "Garden Walk"})
		require.NoError(t, err)
		assert.NotEqual(t, "garden-walk", loc2.Slug) // collision resolved with suffix
		assert.Contains(t, loc2.Slug, "garden-walk")
	})
}

func TestLocationService_LoadBlocks(t *testing.T) {
	service, _, dbc, cleanup := setupLocationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create location (which creates a default header block)
	location, err := service.CreateLocation(
		ctx,
		validQuestID(t, dbc),
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
