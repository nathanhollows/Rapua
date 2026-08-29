package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupQuestSettingsRepo(t *testing.T) (repositories.QuestSettingsRepository, db.Transactor, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	return instanceSettingsRepo, transactor, dbc, cleanup
}

func TestQuestSettingsRepository(t *testing.T) {
	repo, _, dbc, cleanup := setupQuestSettingsRepo(t)
	defer cleanup()

	tests := []struct {
		name   string
		setup  func() *models.QuestSettings
		action func(ctx context.Context, repo repositories.QuestSettingsRepository, settings *models.QuestSettings) error
		verify func(ctx context.Context, t *testing.T, settings *models.QuestSettings, err error)
	}{
		{
			name: "Create instance settings successfully",
			setup: func() *models.QuestSettings {
				parents := createTestParents(t, dbc)
				return &models.QuestSettings{
					QuestID:         parents.QuestID,
					EnablePoints:    gofakeit.Bool(),
					ShowLeaderboard: gofakeit.Bool(),
				}
			},
			action: func(ctx context.Context, repo repositories.QuestSettingsRepository, settings *models.QuestSettings) error {
				return repo.Create(ctx, settings)
			},
			verify: func(_ context.Context, t *testing.T, settings *models.QuestSettings, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, settings.QuestID)
			},
		},
		{
			name: "Update instance settings successfully",
			setup: func() *models.QuestSettings {
				parents := createTestParents(t, dbc)
				return &models.QuestSettings{
					QuestID:         parents.QuestID,
					EnablePoints:    gofakeit.Bool(),
					ShowLeaderboard: gofakeit.Bool(),
				}
			},
			action: func(ctx context.Context, repo repositories.QuestSettingsRepository, settings *models.QuestSettings) error {
				// Simulate creation
				_ = repo.Create(ctx, settings)
				return repo.Update(ctx, settings)
			},
			verify: func(_ context.Context, t *testing.T, _ *models.QuestSettings, err error) {
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

func TestQuestSettingsRepository_GetByQuestID(t *testing.T) {
	// Setup test database
	repo, _, dbc, cleanup := setupQuestSettingsRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create valid parent records
	parents := createTestParents(t, dbc)

	// Create test settings
	settings := &models.QuestSettings{
		QuestID:         parents.QuestID,
		ShowTeamCount:   gofakeit.Bool(),
		EnablePoints:    true,
		ShowLeaderboard: gofakeit.Bool(),
	}

	// Create the settings first
	err := repo.Create(ctx, settings)
	require.NoError(t, err)

	tests := []struct {
		name        string
		questID     string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "Get existing settings",
			questID:     settings.QuestID,
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "Get non-existent settings",
			questID:     gofakeit.UUID(),
			expectError: true,
			expectNil:   true,
		},
		{
			name:        "Empty instance ID",
			questID:     "",
			expectError: true,
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, getErr := repo.GetByQuestID(ctx, tt.questID)

			if tt.expectError {
				require.Error(t, getErr)
			} else {
				require.NoError(t, getErr)
			}

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, settings.QuestID, result.QuestID)
				assert.Equal(t, settings.EnablePoints, result.EnablePoints)
			}
		})
	}
}

func TestQuestSettingsRepository_CreateTx(t *testing.T) {
	repo, transactor, dbc, cleanup := setupQuestSettingsRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates instance settings within transaction", func(t *testing.T) {
		parents := createTestParents(t, dbc)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		settings := &models.QuestSettings{
			QuestID:         parents.QuestID,
			EnablePoints:    true,
			ShowLeaderboard: true,
		}

		err = repo.CreateTx(ctx, tx, settings)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify settings were created
		found, err := repo.GetByQuestID(ctx, parents.QuestID)
		require.NoError(t, err)
		assert.Equal(t, settings.QuestID, found.QuestID)
		assert.Equal(t, settings.EnablePoints, found.EnablePoints)
		assert.Equal(t, settings.ShowLeaderboard, found.ShowLeaderboard)
	})

	t.Run("rolls back on transaction failure", func(t *testing.T) {
		parents := createTestParents(t, dbc)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)

		settings := &models.QuestSettings{
			QuestID:      parents.QuestID,
			EnablePoints: false,
		}

		err = repo.CreateTx(ctx, tx, settings)
		require.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify settings were NOT created
		_, err = repo.GetByQuestID(ctx, parents.QuestID)
		require.Error(t, err)
	})

	t.Run("validates required fields", func(t *testing.T) {
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		settings := &models.QuestSettings{
			// Missing QuestID
			EnablePoints: true,
		}

		err = repo.CreateTx(ctx, tx, settings)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance ID is required")
	})
}
