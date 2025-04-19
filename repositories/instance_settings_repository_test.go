package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
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

	var randomCompletionMethod = func() models.CompletionMethod {
		return models.CompletionMethod(gofakeit.Number(0, 1))
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
					CompletionMethod:  randomCompletionMethod(),
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
					CompletionMethod:  randomCompletionMethod(),
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
					CompletionMethod:  randomCompletionMethod(),
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
