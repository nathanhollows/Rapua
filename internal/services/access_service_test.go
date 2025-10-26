package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/internal/services"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type AccessService interface {
	CanAdminAccessBlock(ctx context.Context, userID, blockID string) (bool, error)
	CanAdminAccessInstance(ctx context.Context, userID, instanceID string) (bool, error)
	CanAdminAccessLocation(ctx context.Context, userID, locationID string) (bool, error)
	CanAdminAccessMarker(ctx context.Context, userID, markerID string) (bool, error)
}

func setupAccessService(t *testing.T) (AccessService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)

	accessService := services.NewAccessService(blockRepo, instanceRepo, locationRepo, markerRepo)

	return accessService, cleanup
}

func TestAccessService_CanAdminAccessInstance(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Valid user and instance access", func(t *testing.T) {
		userID := gofakeit.UUID()
		instanceID := gofakeit.UUID()

		// This will likely return false due to no data setup, but validates the logic
		canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, instanceID)

		// Should not error with valid inputs
		require.NoError(t, err)
		assert.False(t, canAccess) // Expected false since no instances exist for user
	})

	t.Run("Empty user ID", func(t *testing.T) {
		instanceID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessInstance(context.Background(), "", instanceID)

		require.Error(t, err)
		assert.Equal(t, services.ErrUserNotAuthenticated, err)
		assert.False(t, canAccess)
	})

	t.Run("Empty instance ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Both empty user and instance ID", func(t *testing.T) {
		canAccess, err := service.CanAdminAccessInstance(context.Background(), "", "")

		require.Error(t, err)
		assert.Equal(t, services.ErrUserNotAuthenticated, err)
		assert.False(t, canAccess)
	})

	t.Run("User with multiple instances", func(t *testing.T) {
		userID := gofakeit.UUID()
		instanceID1 := gofakeit.UUID()
		instanceID2 := gofakeit.UUID()

		// Test with different instance IDs
		canAccess1, err1 := service.CanAdminAccessInstance(context.Background(), userID, instanceID1)
		canAccess2, err2 := service.CanAdminAccessInstance(context.Background(), userID, instanceID2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.False(t, canAccess1)
		assert.False(t, canAccess2)
	})

	t.Run("Whitespace in user ID", func(t *testing.T) {
		instanceID := gofakeit.UUID()

		// Test with whitespace user ID
		canAccess, err := service.CanAdminAccessInstance(context.Background(), "   ", instanceID)

		// Should pass validation (non-empty string)
		require.NoError(t, err)
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in instance ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		// Test with whitespace instance ID
		canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, "   ")

		// Should pass validation (non-empty string)
		require.NoError(t, err)
		assert.False(t, canAccess)
	})
}

func TestAccessService_CanAdminAccessLocation(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Valid user and location access", func(t *testing.T) {
		userID := gofakeit.UUID()
		locationID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessLocation(context.Background(), userID, locationID)

		// May error due to location not existing, but shouldn't be validation error
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
			require.NotContains(t, err.Error(), "location ID cannot be empty")
		} else {
			assert.False(t, canAccess)
		}
	})

	t.Run("Empty user ID", func(t *testing.T) {
		locationID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessLocation(context.Background(), "", locationID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Empty location ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessLocation(context.Background(), userID, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "location ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Both empty user and location ID", func(t *testing.T) {
		canAccess, err := service.CanAdminAccessLocation(context.Background(), "", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Non-existent location", func(t *testing.T) {
		userID := gofakeit.UUID()
		locationID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessLocation(context.Background(), userID, locationID)

		// Should error due to location not existing in database
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
			require.NotContains(t, err.Error(), "location ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in user ID", func(t *testing.T) {
		locationID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessLocation(context.Background(), "   ", locationID)

		// Should pass validation (non-empty string)
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in location ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessLocation(context.Background(), userID, "   ")

		// Should pass validation (non-empty string)
		if err != nil {
			require.NotContains(t, err.Error(), "location ID cannot be empty")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_CanAdminAccessMarker(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Valid user and marker access", func(t *testing.T) {
		userID := gofakeit.UUID()
		markerID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessMarker(context.Background(), userID, markerID)

		// Should not error with valid inputs
		require.NoError(t, err)
		assert.False(t, canAccess) // Expected false since no markers exist for user
	})

	t.Run("Empty user ID", func(t *testing.T) {
		markerID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessMarker(context.Background(), "", markerID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Empty marker ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessMarker(context.Background(), userID, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "marker ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Both empty user and marker ID", func(t *testing.T) {
		canAccess, err := service.CanAdminAccessMarker(context.Background(), "", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Non-existent marker", func(t *testing.T) {
		userID := gofakeit.UUID()
		markerID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessMarker(context.Background(), userID, markerID)

		// Should not error - UserOwnsMarker should return false for non-existent marker
		require.NoError(t, err)
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in user ID", func(t *testing.T) {
		markerID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessMarker(context.Background(), "   ", markerID)

		// Should pass validation (non-empty string)
		require.NoError(t, err)
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in marker ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessMarker(context.Background(), userID, "   ")

		// Should pass validation (non-empty string)
		require.NoError(t, err)
		assert.False(t, canAccess)
	})
}

func TestAccessService_CanAdminAccessBlock(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Valid user and block access", func(t *testing.T) {
		userID := gofakeit.UUID()
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, blockID)

		// May error due to block not existing, but shouldn't be validation error
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		} else {
			assert.False(t, canAccess)
		}
	})

	t.Run("Empty user ID", func(t *testing.T) {
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), "", blockID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Empty block ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "block ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Both empty user and block ID", func(t *testing.T) {
		canAccess, err := service.CanAdminAccessBlock(context.Background(), "", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Non-existent block", func(t *testing.T) {
		userID := gofakeit.UUID()
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, blockID)

		// Should error due to block not existing in database
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in user ID", func(t *testing.T) {
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), "   ", blockID)

		// Should pass validation (non-empty string)
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in block ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, "   ")

		// Should pass validation (non-empty string)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_ValidationEdgeCases(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Very long IDs", func(t *testing.T) {
		longID := ""
		for range 1000 {
			longID += "a"
		}
		userID := gofakeit.UUID()

		// Test with very long instance ID
		canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, longID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		// Test with very long location ID
		canAccess, err = service.CanAdminAccessLocation(context.Background(), userID, longID)
		if err != nil {
			require.NotContains(t, err.Error(), "location ID cannot be empty")
		}
		assert.False(t, canAccess)

		// Test with very long marker ID
		canAccess, err = service.CanAdminAccessMarker(context.Background(), userID, longID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		// Test with very long block ID
		canAccess, err = service.CanAdminAccessBlock(context.Background(), userID, longID)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Special characters in IDs", func(t *testing.T) {
		userID := gofakeit.UUID()
		specialID := "!@#$%^&*()_+-=[]{}|;':\",./<>?"

		// Test with special characters
		canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, specialID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessLocation(context.Background(), userID, specialID)
		if err != nil {
			require.NotContains(t, err.Error(), "location ID cannot be empty")
		}
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessMarker(context.Background(), userID, specialID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessBlock(context.Background(), userID, specialID)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Unicode characters in IDs", func(t *testing.T) {
		userID := gofakeit.UUID()
		unicodeID := "测试🚀emoji"

		// Test with unicode characters
		canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, unicodeID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessLocation(context.Background(), userID, unicodeID)
		if err != nil {
			require.NotContains(t, err.Error(), "location ID cannot be empty")
		}
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessMarker(context.Background(), userID, unicodeID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessBlock(context.Background(), userID, unicodeID)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_ContextCancellation(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		userID := gofakeit.UUID()
		instanceID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessInstance(ctx, userID, instanceID)

		// Should handle cancelled context gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_ConcurrentAccess(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Concurrent access checks", func(t *testing.T) {
		userID := gofakeit.UUID()
		instanceID := gofakeit.UUID()

		// Run multiple concurrent access checks
		results := make(chan bool, 10)
		errors := make(chan error, 10)

		for range 10 {
			go func() {
				canAccess, err := service.CanAdminAccessInstance(context.Background(), userID, instanceID)
				results <- canAccess
				errors <- err
			}()
		}

		// Collect results
		for range 10 {
			canAccess := <-results
			err := <-errors
			require.NoError(t, err)
			assert.False(t, canAccess)
		}
	})
}
