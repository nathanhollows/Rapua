package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/db"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupInstanceSettingsRepo(t *testing.T) (repositories.InstanceSettingsRepository, db.Transactor, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	return instanceSettingsRepo, transactor, dbc, cleanup
}

func TestInstanceSettingsRepository(t *testing.T) {
	repo, _, dbc, cleanup := setupInstanceSettingsRepo(t)
	defer cleanup()

	tests := []struct {
		name   string
		setup  func() *models.InstanceSettings
		action func(ctx context.Context, repo repositories.InstanceSettingsRepository, settings *models.InstanceSettings) error
		verify func(ctx context.Context, t *testing.T, settings *models.InstanceSettings, err error)
	}{
		{
			name: "Create instance settings successfully",
			setup: func() *models.InstanceSettings {
				parents := createTestParents(t, dbc)
				return &models.InstanceSettings{
					InstanceID:        parents.InstanceID,
					MustCheckOut:      gofakeit.Bool(),
					EnablePoints:      gofakeit.Bool(),
					EnableBonusPoints: gofakeit.Bool(),
					ShowLeaderboard:   gofakeit.Bool(),
				}
			},
			action: func(ctx context.Context, repo repositories.InstanceSettingsRepository, settings *models.InstanceSettings) error {
				return repo.Create(ctx, settings)
			},
			verify: func(_ context.Context, t *testing.T, settings *models.InstanceSettings, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, settings.InstanceID)
			},
		},
		{
			name: "Update instance settings successfully",
			setup: func() *models.InstanceSettings {
				parents := createTestParents(t, dbc)
				return &models.InstanceSettings{
					InstanceID:        parents.InstanceID,
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
			verify: func(_ context.Context, t *testing.T, _ *models.InstanceSettings, err error) {
				require.NoError(t, err)
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
	repo, _, dbc, cleanup := setupInstanceSettingsRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create valid parent records
	parents := createTestParents(t, dbc)

	// Create test settings
	settings := &models.InstanceSettings{
		InstanceID:        parents.InstanceID,
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
			result, getErr := repo.GetByInstanceID(ctx, tt.instanceID)

			if tt.expectError {
				require.Error(t, getErr)
			} else {
				require.NoError(t, getErr)
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

func TestInstanceSettingsRepository_CreateTx(t *testing.T) {
	repo, transactor, dbc, cleanup := setupInstanceSettingsRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates instance settings within transaction", func(t *testing.T) {
		parents := createTestParents(t, dbc)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		settings := &models.InstanceSettings{
			InstanceID:      parents.InstanceID,
			EnablePoints:    true,
			ShowLeaderboard: true,
		}

		err = repo.CreateTx(ctx, tx, settings)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify settings were created
		found, err := repo.GetByInstanceID(ctx, parents.InstanceID)
		require.NoError(t, err)
		assert.Equal(t, settings.InstanceID, found.InstanceID)
		assert.Equal(t, settings.EnablePoints, found.EnablePoints)
		assert.Equal(t, settings.ShowLeaderboard, found.ShowLeaderboard)
	})

	t.Run("rolls back on transaction failure", func(t *testing.T) {
		parents := createTestParents(t, dbc)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)

		settings := &models.InstanceSettings{
			InstanceID:   parents.InstanceID,
			EnablePoints: false,
		}

		err = repo.CreateTx(ctx, tx, settings)
		require.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify settings were NOT created
		_, err = repo.GetByInstanceID(ctx, parents.InstanceID)
		require.Error(t, err)
	})

	t.Run("validates required fields", func(t *testing.T) {
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		settings := &models.InstanceSettings{
			// Missing InstanceID
			EnablePoints: true,
		}

		err = repo.CreateTx(ctx, tx, settings)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance ID is required")
	})
}
