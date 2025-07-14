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

func setupLocationStatsService(t *testing.T) (services.LocationStatsService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	locationRepo := repositories.NewLocationRepository(dbc)
	locationStatsService := services.NewLocationStatsService(locationRepo)

	return locationStatsService, cleanup
}

func TestLocationStatsService_IncrementVisitors(t *testing.T) {
	service, cleanup := setupLocationStatsService(t)
	defer cleanup()

	t.Run("Increment visitors successfully", func(t *testing.T) {
		location := &models.Location{
			ID:           gofakeit.UUID(),
			Name:         gofakeit.Name(),
			InstanceID:   gofakeit.UUID(),
			MarkerID:     gofakeit.UUID(),
			Points:       gofakeit.Number(0, 100),
			TotalVisits:  5,
			CurrentCount: 2,
		}

		err := service.IncrementVisitors(context.Background(), location)
		assert.NoError(t, err)
		assert.Equal(t, 6, location.TotalVisits)
		assert.Equal(t, 3, location.CurrentCount)
	})

	t.Run("Increment visitors with nil location", func(t *testing.T) {
		err := service.IncrementVisitors(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "location cannot be nil")
	})

	t.Run("Increment visitors from zero", func(t *testing.T) {
		location := &models.Location{
			ID:           gofakeit.UUID(),
			Name:         gofakeit.Name(),
			InstanceID:   gofakeit.UUID(),
			MarkerID:     gofakeit.UUID(),
			Points:       gofakeit.Number(0, 100),
			TotalVisits:  0,
			CurrentCount: 0,
		}

		err := service.IncrementVisitors(context.Background(), location)
		assert.NoError(t, err)
		assert.Equal(t, 1, location.TotalVisits)
		assert.Equal(t, 1, location.CurrentCount)
	})
}

func TestLocationStatsService_DecrementVisitors(t *testing.T) {
	service, cleanup := setupLocationStatsService(t)
	defer cleanup()

	t.Run("Decrement visitors successfully", func(t *testing.T) {
		location := &models.Location{
			ID:           gofakeit.UUID(),
			Name:         gofakeit.Name(),
			InstanceID:   gofakeit.UUID(),
			MarkerID:     gofakeit.UUID(),
			Points:       gofakeit.Number(0, 100),
			TotalVisits:  5,
			CurrentCount: 3,
		}

		err := service.DecrementVisitors(context.Background(), location)
		assert.NoError(t, err)
		assert.Equal(t, 5, location.TotalVisits) // TotalVisits should not change
		assert.Equal(t, 2, location.CurrentCount)
	})

	t.Run("Decrement visitors with nil location", func(t *testing.T) {
		err := service.DecrementVisitors(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "location cannot be nil")
	})

	t.Run("Decrement visitors when current count is zero", func(t *testing.T) {
		location := &models.Location{
			ID:           gofakeit.UUID(),
			Name:         gofakeit.Name(),
			InstanceID:   gofakeit.UUID(),
			MarkerID:     gofakeit.UUID(),
			Points:       gofakeit.Number(0, 100),
			TotalVisits:  5,
			CurrentCount: 0,
		}

		err := service.DecrementVisitors(context.Background(), location)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current count cannot be negative")
		assert.Equal(t, 0, location.CurrentCount) // Should remain unchanged
	})

	t.Run("Decrement visitors to zero", func(t *testing.T) {
		location := &models.Location{
			ID:           gofakeit.UUID(),
			Name:         gofakeit.Name(),
			InstanceID:   gofakeit.UUID(),
			MarkerID:     gofakeit.UUID(),
			Points:       gofakeit.Number(0, 100),
			TotalVisits:  5,
			CurrentCount: 1,
		}

		err := service.DecrementVisitors(context.Background(), location)
		assert.NoError(t, err)
		assert.Equal(t, 5, location.TotalVisits) // TotalVisits should not change
		assert.Equal(t, 0, location.CurrentCount)
	})
}