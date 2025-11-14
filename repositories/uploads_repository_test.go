package repositories_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUploadsRepository(t *testing.T) (repositories.UploadsRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	uploadsRepository := repositories.NewUploadRepository(dbc)

	return uploadsRepository, transactor, cleanup
}

func TestUploadRepository_Create(t *testing.T) {
	repo, _, cleanup := setupUploadsRepository(t)
	defer cleanup()

	tests := []struct {
		name      string
		upload    *models.Upload
		expectErr bool
	}{
		{
			name:      "Valid Upload",
			upload:    &models.Upload{OriginalURL: "https://cdn.example.com/original.jpg"},
			expectErr: false,
		},
		{
			name:      "Nil Upload",
			upload:    nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), tt.upload)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, tt.upload.ID) // Ensure ID was generated
			}
		})
	}
}

func TestUploadRepository_SearchByCriteria(t *testing.T) {
	repo, _, cleanup := setupUploadsRepository(t)
	defer cleanup()

	tests := []struct {
		name      string
		criteria  map[string]string
		expectErr bool
	}{
		{
			name:      "Valid Search by ID",
			criteria:  map[string]string{"id": uuid.New().String()},
			expectErr: false,
		},
		{
			name:      "Invalid Field",
			criteria:  map[string]string{"invalid_field": "value"},
			expectErr: true,
		},
		{
			name:      "Empty Criteria",
			criteria:  map[string]string{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.SearchByCriteria(context.Background(), tt.criteria)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
