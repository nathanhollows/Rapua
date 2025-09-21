package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupInstanceSettingsRepo(t *testing.T) (repositories.InstanceSettingsRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	return instanceSettingsRepo, transactor, cleanup
}

func TestInstanceSettingsRepository(t *testing.T) {
	repo, transactor, cleanup := setupInstanceSettingsRepo(t)
	defer cleanup()

	var randomNavMode = func() models.NavigationMode {
		return models.NavigationMode(gofakeit.Number(0, 2))
	}

	var randomNavMethod = func() models.NavigationMethod {
		return models.NavigationMethod(gofakeit.Number(0, 3))
	}

	tests := []struct {
		name   string
		setup  func() *models.InstanceSettings
		action func(ctx context.Context, repo repositories.InstanceSettingsRepository, settings *models.InstanceSettings) error
		verify func(ctx context.Context, t *testing.T, settings *models.InstanceSettings, err error)
	}{
		{
			name: "Create instance settings successfully",
			setup: func() *models.InstanceSettings {
				return &models.InstanceSettings{
					InstanceID:        gofakeit.UUID(),
					NavigationMode:    randomNavMode(),
					NavigationMethod:  randomNavMethod(),
					MaxNextLocations:  gofakeit.Number(1, 10),
					MustCheckOut:      gofakeit.Bool(),
					EnablePoints:      gofakeit.Bool(),
					EnableBonusPoints: gofakeit.Bool(),
					ShowLeaderboard:   gofakeit.Bool(),
				}
			},
			action: func(ctx context.Context, repo repositories.InstanceSettingsRepository, settings *models.InstanceSettings) error {
				return repo.Create(ctx, settings)
			},
			verify: func(ctx context.Context, t *testing.T, settings *models.InstanceSettings, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, settings.InstanceID)
			},
		},
		{
			name: "Update instance settings successfully",
			setup: func() *models.InstanceSettings {
				return &models.InstanceSettings{
					InstanceID:        gofakeit.UUID(),
					NavigationMode:    randomNavMode(),
					NavigationMethod:  randomNavMethod(),
					MaxNextLocations:  gofakeit.Number(1, 10),
					MustCheckOut:      gofakeit.Bool(),
					EnablePoints:      gofakeit.Bool(),
					EnableBonusPoints: gofakeit.Bool(),
					ShowLeaderboard:   gofakeit.Bool(),
				}
			},
			action: func(ctx context.Context, repo repositories.InstanceSettingsRepository, settings *models.InstanceSettings) error {
				// Simulate creation
				_ = repo.Create(ctx, settings)
				return repo.Update(ctx, settings)
			},
			verify: func(ctx context.Context, t *testing.T, settings *models.InstanceSettings, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "Delete instance settings successfully",
			setup: func() *models.InstanceSettings {
				return &models.InstanceSettings{
					InstanceID:        gofakeit.UUID(),
					NavigationMode:    randomNavMode(),
					NavigationMethod:  randomNavMethod(),
					MaxNextLocations:  gofakeit.Number(1, 10),
					MustCheckOut:      gofakeit.Bool(),
					EnablePoints:      gofakeit.Bool(),
					EnableBonusPoints: gofakeit.Bool(),
					ShowLeaderboard:   gofakeit.Bool(),
				}
			},
			action: func(ctx context.Context, repo repositories.InstanceSettingsRepository, settings *models.InstanceSettings) error {
				// Simulate creation
				_ = repo.Create(ctx, settings)

				tx, _ := transactor.BeginTx(ctx, &sql.TxOptions{})
				defer (func() {
					if err := tx.Commit(); err != nil {
						t.Error(err)
					}
				})()

				return repo.Delete(ctx, tx, settings.InstanceID)
			},
			verify: func(ctx context.Context, t *testing.T, settings *models.InstanceSettings, err error) {
				assert.NoError(t, err)
				// Optionally, query the database to confirm the instance was deleted
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			settings := tt.setup()

			// Act
			err := tt.action(ctx, repo, settings)

			// Assert
			tt.verify(ctx, t, settings, err)
		})
	}
}

func TestInstanceSettingsRepository_GetByInstanceID(t *testing.T) {
	// Setup test database
	repo, _, cleanup := setupInstanceSettingsRepo(t)
	defer cleanup()

	ctx := context.Background()

	var randomNavMode = func() models.NavigationMode {
		return models.NavigationMode(gofakeit.Number(0, 2))
	}

	var randomNavMethod = func() models.NavigationMethod {
		return models.NavigationMethod(gofakeit.Number(0, 3))
	}

	// Create test settings
	settings := &models.InstanceSettings{
		InstanceID:        gofakeit.UUID(),
		NavigationMode:    randomNavMode(),
		NavigationMethod:  randomNavMethod(),
		MaxNextLocations:  gofakeit.Number(1, 10),
		MustCheckOut:      gofakeit.Bool(),
		ShowTeamCount:     gofakeit.Bool(),
		EnablePoints:      true,
		EnableBonusPoints: gofakeit.Bool(),
		ShowLeaderboard:   gofakeit.Bool(),
	}

	// Create the settings first
	err := repo.Create(ctx, settings)
	require.NoError(t, err)

	tests := []struct {
		name        string
		instanceID  string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "Get existing settings",
			instanceID:  settings.InstanceID,
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "Get non-existent settings",
			instanceID:  gofakeit.UUID(),
			expectError: true,
			expectNil:   true,
		},
		{
			name:        "Empty instance ID",
			instanceID:  "",
			expectError: true,
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByInstanceID(ctx, tt.instanceID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, settings.InstanceID, result.InstanceID)
				assert.Equal(t, settings.EnablePoints, result.EnablePoints)
			}
		})
	}
}
