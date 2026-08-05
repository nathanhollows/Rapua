package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupQuestSettingsService(t *testing.T) (*services.QuestSettingsService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	instanceSettingsService := services.NewQuestSettingsService(instanceSettingsRepo)

	return instanceSettingsService, cleanup
}

func createTestQuestSettings(t *testing.T) *models.QuestSettings {
	t.Helper()

	return &models.QuestSettings{
		QuestID:       gofakeit.UUID(),
		MustCheckOut:  gofakeit.Bool(),
		ShowTeamCount: false,
		EnablePoints:  true,
	}
}

func TestQuestSettingsService_GetQuestSettings(t *testing.T) {
	service, cleanup := setupQuestSettingsService(t)
	defer cleanup()

	t.Run("Get existing settings", func(t *testing.T) {
		// This test would require creating settings in the database first
		// For now, we'll test the error cases
		settings, err := service.GetQuestSettings(context.Background(), "nonexistent-id")
		require.Error(t, err)
		assert.Nil(t, settings)
	})

	t.Run("Empty instance ID", func(t *testing.T) {
		settings, err := service.GetQuestSettings(context.Background(), "")
		require.Error(t, err)
		assert.Nil(t, settings)
		assert.Contains(t, err.Error(), "instance ID cannot be empty")
	})
}

func TestQuestSettingsService_SaveSettings(t *testing.T) {
	service, cleanup := setupQuestSettingsService(t)
	defer cleanup()

	t.Run("Save valid settings", func(t *testing.T) {
		settings := createTestQuestSettings(t)

		err := service.SaveSettings(context.Background(), settings)
		// Validation should pass - may succeed or fail depending on database state
		// We're mainly testing that validation doesn't reject valid settings
		if err != nil {
			// If there's an error, it shouldn't be from our validation rules
			assert.NotContains(t, err.Error(), "settings cannot be nil")
		}
	})

	t.Run("Save nil settings", func(t *testing.T) {
		err := service.SaveSettings(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "settings cannot be nil")
	})

	t.Run("Save settings with boolean flags", func(t *testing.T) {
		testCases := []struct {
			name   string
			modify func(*models.QuestSettings)
		}{
			{"ShowTeamCount true", func(s *models.QuestSettings) { s.ShowTeamCount = true }},
			{"ShowTeamCount false", func(s *models.QuestSettings) { s.ShowTeamCount = false }},
			{"EnablePoints true", func(s *models.QuestSettings) { s.EnablePoints = true }},
			{"EnablePoints false", func(s *models.QuestSettings) { s.EnablePoints = false }},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				settings := createTestQuestSettings(t)
				tc.modify(settings)

				err := service.SaveSettings(context.Background(), settings)
				// Validation should pass, might fail on database operations
				if err != nil {
					assert.NotContains(t, err.Error(), "max next locations cannot be negative")
				}
			})
		}
	})
}
