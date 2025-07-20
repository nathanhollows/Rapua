package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func setupMarkerRepo(t *testing.T) (repositories.MarkerRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	markerRepo := repositories.NewMarkerRepository(dbc)
	return markerRepo, transactor, cleanup
}

func TestMarkerRepository_Create(t *testing.T) {
	repo, _, cleanup := setupMarkerRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() (models.Marker, error)
		action      func(marker models.Marker) error
		assertion   func(err error)
		cleanupFunc func(marker models.Marker)
	}{
		{
			name: "Create new marker",
			setup: func() (models.Marker, error) {
				return models.Marker{
					Code: gofakeit.UUID(),
					Lat:  gofakeit.Latitude(),
					Lng:  gofakeit.Longitude(),
					Name: gofakeit.Name(),
				}, nil
			},
			action: func(marker models.Marker) error {
				return repo.Create(context.Background(), &marker)
			},
			assertion: func(err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				err := repo.Delete(context.Background(), marker.Code)
				assert.NoError(t, err)
			},
		},
		{
			name: "Empty marker",
			setup: func() (models.Marker, error) {
				return models.Marker{}, nil
			},
			action: func(marker models.Marker) error {
				return repo.Create(context.Background(), &marker)
			},
			assertion: func(err error) {
				assert.Error(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				err := repo.Delete(context.Background(), marker.Code)
				assert.NoError(t, err)
			},
		},
		{
			name: "No name",
			setup: func() (models.Marker, error) {
				return models.Marker{
					Code: gofakeit.UUID(),
					Lat:  gofakeit.Latitude(),
					Lng:  gofakeit.Longitude(),
				}, nil
			},
			action: func(marker models.Marker) error {
				return repo.Create(context.Background(), &marker)
			},
			assertion: func(err error) {
				assert.Error(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				err := repo.Delete(context.Background(), marker.Code)
				assert.NoError(t, err)
			},
		},
		{
			name: "No coordinates",
			setup: func() (models.Marker, error) {
				return models.Marker{
					Code: gofakeit.UUID(),
					Name: gofakeit.Name(),
				}, nil
			},
			action: func(marker models.Marker) error {
				return repo.Create(context.Background(), &marker)
			},
			assertion: func(err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				err := repo.Delete(context.Background(), marker.Code)
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, err := tt.setup()
			assert.NoError(t, err)

			err = tt.action(marker)
			tt.assertion(err)

			tt.cleanupFunc(marker)
		})
	}
}

func TestMarkerRepository_GetByCode(t *testing.T) {
	repo, _, cleanup := setupMarkerRepo(t)
	defer cleanup()

	marker := models.Marker{
		Lat:  gofakeit.Latitude(),
		Lng:  gofakeit.Longitude(),
		Name: gofakeit.Name(),
	}
	err := repo.Create(context.Background(), &marker)
	assert.NoError(t, err)

	foundMarker, err := repo.GetByCode(context.Background(), marker.Code)
	assert.NoError(t, err)
	assert.Equal(t, marker, *foundMarker)
}

// NOTE: FindNotInInstance is not yet tested here.
// TODO: Add test cases for FindNotInInstance. This requires more setup and teardown logic as it involves relationships between markers and locations.

func TestMarkerRepository_Update(t *testing.T) {
	repo, _, cleanup := setupMarkerRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() (models.Marker, error)
		action      func(marker models.Marker) error
		assertion   func(err error)
		cleanupFunc func(marker models.Marker)
	}{
		{
			name: "Update marker",
			setup: func() (models.Marker, error) {
				marker := models.Marker{
					Lat:  gofakeit.Latitude(),
					Lng:  gofakeit.Longitude(),
					Name: "Initial Name",
				}
				if err := repo.Create(context.Background(), &marker); err != nil {
					return models.Marker{}, err
				}
				return marker, nil
			},
			action: func(marker models.Marker) error {
				marker.Name = "Updated Name"
				return repo.Update(context.Background(), &marker)
			},
			assertion: func(err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				err := repo.Delete(context.Background(), marker.Code)
				assert.NoError(t, err)
			},
		},
		{
			name: "Empty marker",
			setup: func() (models.Marker, error) {
				return models.Marker{}, nil
			},
			action: func(marker models.Marker) error {
				return repo.Update(context.Background(), &marker)
			},
			assertion: func(err error) {
				assert.Error(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				// No cleanup needed as marker was not created
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, err := tt.setup()
			assert.NoError(t, err)

			err = tt.action(marker)
			tt.assertion(err)

			tt.cleanupFunc(marker)
		})
	}
}

func TestMarkerRepository_UpdateCoords(t *testing.T) {
	repo, _, cleanup := setupMarkerRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() (models.Marker, error)
		action      func(marker models.Marker) error
		assertion   func(marker models.Marker, err error)
		cleanupFunc func(marker models.Marker)
	}{
		{
			name: "Update marker coordinates",
			setup: func() (models.Marker, error) {
				marker := models.Marker{
					Code: gofakeit.UUID(),
					Lat:  gofakeit.Latitude(),
					Lng:  gofakeit.Longitude(),
					Name: "Initial Marker",
				}
				if err := repo.Create(context.Background(), &marker); err != nil {
					return models.Marker{}, err
				}
				return marker, nil
			},
			action: func(marker models.Marker) error {
				newLat, newLng := gofakeit.Latitude(), gofakeit.Longitude()
				err := repo.UpdateCoords(context.Background(), &marker, newLat, newLng)
				if err == nil {
					// Update the marker object with new coordinates for assertion
					marker.Lat = newLat
					marker.Lng = newLng
				}
				return err
			},
			assertion: func(marker models.Marker, err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				err := repo.Delete(context.Background(), marker.Code)
				assert.NoError(t, err)
			},
		},
		{
			name: "Empty marker",
			setup: func() (models.Marker, error) {
				return models.Marker{}, nil
			},
			action: func(marker models.Marker) error {
				return repo.UpdateCoords(context.Background(), &marker, gofakeit.Latitude(), gofakeit.Longitude())
			},
			assertion: func(marker models.Marker, err error) {
				assert.Error(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				// No cleanup needed as marker was not created
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, err := tt.setup()
			assert.NoError(t, err)

			err = tt.action(marker)
			tt.assertion(marker, err)

			tt.cleanupFunc(marker)
		})
	}
}

func TestMarkerRepository_Delete(t *testing.T) {
	repo, _, cleanup := setupMarkerRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() (models.Marker, error)
		action      func(markerCode string) error
		assertion   func(err error)
		cleanupFunc func(marker models.Marker)
	}{
		{
			name: "Delete existing marker",
			setup: func() (models.Marker, error) {
				marker := models.Marker{
					Code: gofakeit.UUID(),
					Lat:  gofakeit.Latitude(),
					Lng:  gofakeit.Longitude(),
					Name: "To be deleted",
				}
				if err := repo.Create(context.Background(), &marker); err != nil {
					return models.Marker{}, err
				}
				return marker, nil
			},
			action: func(markerCode string) error {
				return repo.Delete(context.Background(), markerCode)
			},
			assertion: func(err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(marker models.Marker) {
				// No cleanup needed as marker is deleted
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, err := tt.setup()
			assert.NoError(t, err)

			err = tt.action(marker.Code)
			tt.assertion(err)

			tt.cleanupFunc(marker)
		})
	}
}

func TestMarkerRepository_DeleteUnused(t *testing.T) {
	repo, transactor, cleanup := setupMarkerRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() (*bun.Tx, error)
		action      func(tx *bun.Tx) error
		assertion   func(err error)
		cleanupFunc func()
	}{
		{
			name: "Delete unused markers",
			setup: func() (*bun.Tx, error) {
				tx, err := transactor.BeginTx(context.Background(), nil)
				if err != nil {
					return nil, err
				}

				// Insert marker that would be considered unused
				marker := models.Marker{
					Code: gofakeit.UUID(),
					Lat:  gofakeit.Latitude(),
					Lng:  gofakeit.Longitude(),
					Name: "Unused Marker",
				}
				fields := context.TODO()
				err = repo.Create(fields, &marker)
				return tx, err
			},
			action: func(tx *bun.Tx) error {
				return repo.DeleteUnused(context.Background(), tx)
			},
			assertion: func(err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func() {
				// Assume the function deletes all unused markers
				// No specific marker cleanup since all unused markers are assumed deleted
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := tt.setup()
			assert.NoError(t, err)

			err = tt.action(tx)
			tt.assertion(err)

			if err == nil {
				tt.cleanupFunc()
			}
			rollbackErr := tx.Rollback() // Assuming rollback for test isolation
			assert.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		})
	}
}

// NOTE: IsShared and UserOwnsMarker are not yet tested here.
// TODO: Add test cases for IsShared and UserOwnsMarker. These require more setup and teardown logic as they involve relationships between markers and locations.
