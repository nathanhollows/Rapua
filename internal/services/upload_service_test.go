package services_test

import (
	"context"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUploadStorage struct{}

func (m *mockUploadStorage) Upload(
	ctx context.Context,
	file multipart.File,
	filename string,
) (map[string]string, string, error) {
	if filename == "error.jpg" {
		return nil, "", errors.New("storage upload error")
	}
	return map[string]string{"original": "https://cdn.example.com/" + filename}, "delete-token", nil
}

func (m *mockUploadStorage) Type() string {
	return "mock"
}

func setupUploadService(t *testing.T) (services.UploadService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	uploadsRepository := repositories.NewUploadRepository(dbc)
	mockStorage := &mockUploadStorage{}

	uploadService := services.NewUploadService(uploadsRepository, mockStorage)
	return *uploadService, cleanup
}

func TestUploadService_UploadFile(t *testing.T) {
	svc, cleanup := setupUploadService(t)
	defer cleanup()

	tests := []struct {
		name      string
		filename  string
		fileType  string
		expectErr bool
	}{
		{
			name:      "Valid image upload",
			filename:  "test.jpg",
			fileType:  "image/jpeg",
			expectErr: false,
		},
		{
			name:      "Invalid file type",
			filename:  "test.exe",
			fileType:  "application/x-msdownload",
			expectErr: true,
		},
		{
			name:      "Storage error",
			filename:  "error.jpg",
			fileType:  "image/jpeg",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileHeader := &multipart.FileHeader{
				Filename: tt.filename,
				Header:   map[string][]string{"Content-Type": {tt.fileType}},
			}
			result, err := svc.UploadFile(context.Background(), nil, fileHeader, services.UploadMetadata{})

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, "https://cdn.example.com/"+tt.filename, result.OriginalURL)
			}
		})
	}
}

func TestUploadService_Search(t *testing.T) {
	svc, cleanup := setupUploadService(t)
	defer cleanup()

	tests := []struct {
		name      string
		filters   map[string]string
		expectErr bool
	}{
		{
			name:      "Search by ID",
			filters:   map[string]string{"id": "123"},
			expectErr: false,
		},
		{
			name:      "Search by LocationID",
			filters:   map[string]string{"location_id": "456"},
			expectErr: false,
		},
		{
			name:      "Search by InstanceID and TeamID",
			filters:   map[string]string{"instance_id": "789", "team_code": "team1"},
			expectErr: false,
		},
		{
			name:      "Search by invalid field",
			filters:   map[string]string{"invalid_field": "value"},
			expectErr: true,
		},
		{
			name:      "Empty filters",
			filters:   map[string]string{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Search(context.Background(), tt.filters)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
