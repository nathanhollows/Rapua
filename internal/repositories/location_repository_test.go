package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupLocationRepo(t *testing.T) (repositories.LocationRepository, db.Transactor, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	locationRepo := repositories.NewLocationRepository(dbc)
	return locationRepo, transactor, dbc, cleanup
}

func TestLocationRepository_GetByInstanceAndSlug(t *testing.T) {
	repo, _, dbc, cleanup := setupLocationRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	slug := gofakeit.Word()

	location := &models.Location{
		Name:       gofakeit.Name(),
		Slug:       slug,
		InstanceID: parents.InstanceID,
		MarkerID:   parents.MarkerCode,
	}
	err := repo.Create(ctx, location)
	require.NoError(t, err)

	t.Run("found by slug", func(t *testing.T) {
		found, findErr := repo.GetByInstanceAndSlug(ctx, parents.InstanceID, slug)
		require.NoError(t, findErr)
		assert.Equal(t, location.ID, found.ID)
		assert.Equal(t, slug, found.Slug)
	})

	t.Run("not found with wrong slug", func(t *testing.T) {
		_, findErr := repo.GetByInstanceAndSlug(ctx, parents.InstanceID, gofakeit.Word())
		require.Error(t, findErr)
	})

	t.Run("not found with wrong instance", func(t *testing.T) {
		_, findErr := repo.GetByInstanceAndSlug(ctx, gofakeit.UUID(), slug)
		require.Error(t, findErr)
	})

	t.Run("same slug in different instances is allowed", func(t *testing.T) {
		otherParents := createTestParents(t, dbc)
		other := &models.Location{
			Name:       gofakeit.Name(),
			Slug:       slug,
			InstanceID: otherParents.InstanceID,
			MarkerID:   otherParents.MarkerCode,
		}
		createErr := repo.Create(ctx, other)
		require.NoError(t, createErr)

		found, findErr := repo.GetByInstanceAndSlug(ctx, otherParents.InstanceID, slug)
		require.NoError(t, findErr)
		assert.Equal(t, other.ID, found.ID)
	})
}

func TestLocationRepository_CreateTx(t *testing.T) {
	repo, transactor, dbc, cleanup := setupLocationRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates location within transaction", func(t *testing.T) {
		parents := createTestParents(t, dbc)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		location := &models.Location{
			Name:       gofakeit.Word(),
			InstanceID: parents.InstanceID,
			MarkerID:   parents.MarkerCode,
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
		parents := createTestParents(t, dbc)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)

		location := &models.Location{
			Name:       gofakeit.Word(),
			InstanceID: parents.InstanceID,
			MarkerID:   parents.MarkerCode,
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
