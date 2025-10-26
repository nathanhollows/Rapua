package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/db"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLocationRepo(t *testing.T) (repositories.LocationRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	locationRepo := repositories.NewLocationRepository(dbc)
	return locationRepo, transactor, cleanup
}

func TestLocationRepository_CreateTx(t *testing.T) {
	repo, transactor, cleanup := setupLocationRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates location within transaction", func(t *testing.T) {
		// Create instance first
		instanceID := gofakeit.UUID()
		markerID := gofakeit.UUID()

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		location := &models.Location{
			Name:       gofakeit.Word(),
			InstanceID: instanceID,
			MarkerID:   markerID,
			Points:     100,
		}

		err = repo.CreateTx(ctx, tx, location)
		require.NoError(t, err)
		assert.NotEmpty(t, location.ID, "ID should be generated")

		err = tx.Commit()
		require.NoError(t, err)

		// Verify location was created
		found, err := repo.GetByID(ctx, location.ID)
		require.NoError(t, err)
		assert.Equal(t, location.Name, found.Name)
		assert.Equal(t, location.InstanceID, found.InstanceID)
		assert.Equal(t, location.MarkerID, found.MarkerID)
	})

	t.Run("rolls back on transaction failure", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		markerID := gofakeit.UUID()

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)

		location := &models.Location{
			Name:       gofakeit.Word(),
			InstanceID: instanceID,
			MarkerID:   markerID,
			Points:     50,
		}

		err = repo.CreateTx(ctx, tx, location)
		require.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify location was NOT created
		_, err = repo.GetByID(ctx, location.ID)
		require.Error(t, err)
	})
}
