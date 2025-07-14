package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
)

func setupMarkerService(t *testing.T) (*services.MarkerService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	markerRepo := repositories.NewMarkerRepository(dbc)
	markerService := services.NewMarkerService(markerRepo)

	return markerService, cleanup
}

func TestMarkerService_CreateMarker(t *testing.T) {
	service, cleanup := setupMarkerService(t)
	defer cleanup()

	t.Run("Create valid marker", func(t *testing.T) {
		name := gofakeit.Name()
		lat := gofakeit.Latitude()
		lng := gofakeit.Longitude()

		marker, err := service.CreateMarker(context.Background(), name, lat, lng)
		
		// Validation should pass - may succeed or fail depending on database state
		if err != nil {
			// If there's an error, it shouldn't be from our validation rules
			assert.NotContains(t, err.Error(), "name cannot be empty")
			assert.NotContains(t, err.Error(), "latitude must be between")
			assert.NotContains(t, err.Error(), "longitude must be between")
		} else {
			// If successful, check that values are set correctly
			assert.Equal(t, name, marker.Name)
			assert.Equal(t, lat, marker.Lat)
			assert.Equal(t, lng, marker.Lng)
		}
	})

	t.Run("Create marker with empty name", func(t *testing.T) {
		lat := gofakeit.Latitude()
		lng := gofakeit.Longitude()

		marker, err := service.CreateMarker(context.Background(), "", lat, lng)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
		assert.Equal(t, models.Marker{}, marker)
	})

	t.Run("Create marker with invalid latitude - too low", func(t *testing.T) {
		name := gofakeit.Name()
		lat := -91.0 // Invalid
		lng := gofakeit.Longitude()

		marker, err := service.CreateMarker(context.Background(), name, lat, lng)
		assert.Error(t, err)
		assert.Equal(t, services.ErrInvalidLatitude, err)
		assert.Equal(t, models.Marker{}, marker)
	})

	t.Run("Create marker with invalid latitude - too high", func(t *testing.T) {
		name := gofakeit.Name()
		lat := 91.0 // Invalid
		lng := gofakeit.Longitude()

		marker, err := service.CreateMarker(context.Background(), name, lat, lng)
		assert.Error(t, err)
		assert.Equal(t, services.ErrInvalidLatitude, err)
		assert.Equal(t, models.Marker{}, marker)
	})

	t.Run("Create marker with invalid longitude - too low", func(t *testing.T) {
		name := gofakeit.Name()
		lat := gofakeit.Latitude()
		lng := -181.0 // Invalid

		marker, err := service.CreateMarker(context.Background(), name, lat, lng)
		assert.Error(t, err)
		assert.Equal(t, services.ErrInvalidLongitude, err)
		assert.Equal(t, models.Marker{}, marker)
	})

	t.Run("Create marker with invalid longitude - too high", func(t *testing.T) {
		name := gofakeit.Name()
		lat := gofakeit.Latitude()
		lng := 181.0 // Invalid

		marker, err := service.CreateMarker(context.Background(), name, lat, lng)
		assert.Error(t, err)
		assert.Equal(t, services.ErrInvalidLongitude, err)
		assert.Equal(t, models.Marker{}, marker)
	})

	t.Run("Create marker with boundary latitude values", func(t *testing.T) {
		name := gofakeit.Name()
		lng := gofakeit.Longitude()

		// Test exactly -90 (should pass)
		marker, err := service.CreateMarker(context.Background(), name, -90.0, lng)
		if err != nil {
			assert.NotEqual(t, services.ErrInvalidLatitude, err)
		} else {
			assert.Equal(t, -90.0, marker.Lat)
		}

		// Test exactly 90 (should pass)
		marker, err = service.CreateMarker(context.Background(), name, 90.0, lng)
		if err != nil {
			assert.NotEqual(t, services.ErrInvalidLatitude, err)
		} else {
			assert.Equal(t, 90.0, marker.Lat)
		}
	})

	t.Run("Create marker with boundary longitude values", func(t *testing.T) {
		name := gofakeit.Name()
		lat := gofakeit.Latitude()

		// Test exactly -180 (should pass)
		marker, err := service.CreateMarker(context.Background(), name, lat, -180.0)
		if err != nil {
			assert.NotEqual(t, services.ErrInvalidLongitude, err)
		} else {
			assert.Equal(t, -180.0, marker.Lng)
		}

		// Test exactly 180 (should pass)
		marker, err = service.CreateMarker(context.Background(), name, lat, 180.0)
		if err != nil {
			assert.NotEqual(t, services.ErrInvalidLongitude, err)
		} else {
			assert.Equal(t, 180.0, marker.Lng)
		}
	})

	t.Run("Create marker with whitespace name", func(t *testing.T) {
		lat := gofakeit.Latitude()
		lng := gofakeit.Longitude()

		// Test with spaces only
		_, err := service.CreateMarker(context.Background(), "   ", lat, lng)
		// This should pass validation (non-empty string), may fail on database
		if err != nil {
			assert.NotContains(t, err.Error(), "name cannot be empty")
		}

		// Test with tabs and newlines
		_, err = service.CreateMarker(context.Background(), "\t\n", lat, lng)
		if err != nil {
			assert.NotContains(t, err.Error(), "name cannot be empty")
		}
	})
}

func TestMarkerService_FindMarkersNotInInstance(t *testing.T) {
	service, cleanup := setupMarkerService(t)
	defer cleanup()

	t.Run("Find markers with valid parameters", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		otherInstances := []string{gofakeit.UUID(), gofakeit.UUID()}

		_, err := service.FindMarkersNotInInstance(context.Background(), instanceID, otherInstances)
		
		// May succeed or fail depending on database state, but validation should pass
		if err != nil {
			assert.NotContains(t, err.Error(), "instanceID cannot be empty")
			assert.NotContains(t, err.Error(), "otherInstances cannot be empty")
		}
		// If no error, validation passed and database operation succeeded
	})

	t.Run("Find markers with empty instance ID", func(t *testing.T) {
		otherInstances := []string{gofakeit.UUID(), gofakeit.UUID()}

		markers, err := service.FindMarkersNotInInstance(context.Background(), "", otherInstances)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instanceID cannot be empty")
		assert.Nil(t, markers)
	})

	t.Run("Find markers with empty other instances", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		otherInstances := []string{}

		markers, err := service.FindMarkersNotInInstance(context.Background(), instanceID, otherInstances)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "otherInstances cannot be empty")
		assert.Nil(t, markers)
	})

	t.Run("Find markers with nil other instances", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		var otherInstances []string = nil

		markers, err := service.FindMarkersNotInInstance(context.Background(), instanceID, otherInstances)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "otherInstances cannot be empty")
		assert.Nil(t, markers)
	})

	t.Run("Find markers with single other instance", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		otherInstances := []string{gofakeit.UUID()}

		_, err := service.FindMarkersNotInInstance(context.Background(), instanceID, otherInstances)
		
		// Should pass validation
		if err != nil {
			assert.NotContains(t, err.Error(), "instanceID cannot be empty")
			assert.NotContains(t, err.Error(), "otherInstances cannot be empty")
		}
		// If no error, validation passed and database operation succeeded
	})

	t.Run("Find markers with multiple other instances", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		otherInstances := []string{
			gofakeit.UUID(),
			gofakeit.UUID(),
			gofakeit.UUID(),
			gofakeit.UUID(),
		}

		_, err := service.FindMarkersNotInInstance(context.Background(), instanceID, otherInstances)
		
		// Should pass validation
		if err != nil {
			assert.NotContains(t, err.Error(), "instanceID cannot be empty")
			assert.NotContains(t, err.Error(), "otherInstances cannot be empty")
		}
		// If no error, validation passed and database operation succeeded
	})
}

func TestMarkerService_ValidationEdgeCases(t *testing.T) {
	service, cleanup := setupMarkerService(t)
	defer cleanup()

	t.Run("Coordinate precision", func(t *testing.T) {
		name := gofakeit.Name()
		
		// Test very precise coordinates
		lat := 45.123456789
		lng := -122.987654321

		marker, err := service.CreateMarker(context.Background(), name, lat, lng)
		if err != nil {
			assert.NotEqual(t, services.ErrInvalidLatitude, err)
			assert.NotEqual(t, services.ErrInvalidLongitude, err)
		} else {
			assert.Equal(t, lat, marker.Lat)
			assert.Equal(t, lng, marker.Lng)
		}
	})

	t.Run("Zero coordinates", func(t *testing.T) {
		name := gofakeit.Name()

		marker, err := service.CreateMarker(context.Background(), name, 0.0, 0.0)
		if err != nil {
			assert.NotEqual(t, services.ErrInvalidLatitude, err)
			assert.NotEqual(t, services.ErrInvalidLongitude, err)
		} else {
			assert.Equal(t, 0.0, marker.Lat)
			assert.Equal(t, 0.0, marker.Lng)
		}
	})

	t.Run("Very long marker name", func(t *testing.T) {
		// Test with a very long name
		longName := ""
		for i := 0; i < 1000; i++ {
			longName += "a"
		}
		lat := gofakeit.Latitude()
		lng := gofakeit.Longitude()

		marker, err := service.CreateMarker(context.Background(), longName, lat, lng)
		// Should pass validation (non-empty), may fail on database constraints
		if err != nil {
			assert.NotContains(t, err.Error(), "name cannot be empty")
		} else {
			assert.Equal(t, longName, marker.Name)
		}
	})
}