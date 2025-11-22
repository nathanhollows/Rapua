package players

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUploadService struct {
	uploadedFile     multipart.File
	uploadedHeader   *multipart.FileHeader
	uploadedMetadata services.UploadMetadata
	returnUpload     *models.Upload
	returnError      error
}

func (m *mockUploadService) UploadFile(
	ctx context.Context,
	file multipart.File,
	fileHeader *multipart.FileHeader,
	data services.UploadMetadata,
) (*models.Upload, error) {
	m.uploadedFile = file
	m.uploadedHeader = fileHeader
	m.uploadedMetadata = data
	if m.returnError != nil {
		return nil, m.returnError
	}
	return m.returnUpload, nil
}

func TestPlayerHandler_UploadImage_Success(t *testing.T) {
	// Create a mock upload service
	mockService := &mockUploadService{
		returnUpload: &models.Upload{
			ID:          "upload-123",
			OriginalURL: "https://example.com/uploads/test.jpg",
			InstanceID:  "instance-123",
			TeamCode:    "TEAM1",
			BlockID:     "block-456",
			LocationID:  "location-789",
		},
	}

	// Create handler with mock service
	handler := &PlayerHandler{
		logger:        slog.Default(),
		uploadService: mockService,
	}

	// Create a test team
	team := &models.Team{
		Code:       "TEAM1",
		InstanceID: "instance-123",
	}

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	fileWriter, err := writer.CreateFormFile("file", "test.jpg")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake image content"))
	require.NoError(t, err)

	// Add metadata
	err = writer.WriteField("block_id", "block-456")
	require.NoError(t, err)
	err = writer.WriteField("location_id", "location-789")
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add team to context
	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	// Create response recorder
	w := httptest.NewRecorder()

	// Call handler
	handler.UploadImage(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify response body
	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/uploads/test.jpg", response["url"])

	// Verify metadata was passed correctly
	assert.Equal(t, "instance-123", mockService.uploadedMetadata.InstanceID)
	assert.Equal(t, "TEAM1", mockService.uploadedMetadata.TeamID)
	assert.Equal(t, "block-456", mockService.uploadedMetadata.BlockID)
	assert.Equal(t, "location-789", mockService.uploadedMetadata.LocationID)
}

func TestPlayerHandler_UploadImage_NoFile(t *testing.T) {
	mockService := &mockUploadService{}
	handler := &PlayerHandler{
		logger:        slog.Default(),
		uploadService: mockService,
	}

	team := &models.Team{
		Code:       "TEAM1",
		InstanceID: "instance-123",
	}

	// Create multipart form data without file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UploadImage(w, req)

	// handleError returns 200 with toast, verify it's not a successful JSON response
	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	// Should fail to decode as JSON or not have a "url" field
	if err == nil {
		assert.Empty(t, response["url"])
	}
}

func TestPlayerHandler_UploadImage_NoTeamInContext(t *testing.T) {
	mockService := &mockUploadService{}
	handler := &PlayerHandler{
		logger:        slog.Default(),
		uploadService: mockService,
	}

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test.jpg")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake image content"))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// No team in context

	w := httptest.NewRecorder()
	handler.UploadImage(w, req)

	// handleError returns 200 with toast, verify it's not a successful JSON response
	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	if err == nil {
		assert.Empty(t, response["url"])
	}
}

func TestPlayerHandler_UploadImage_ServiceError(t *testing.T) {
	mockService := &mockUploadService{
		returnError: assert.AnError,
	}

	handler := &PlayerHandler{
		logger:        slog.Default(),
		uploadService: mockService,
	}

	team := &models.Team{
		Code:       "TEAM1",
		InstanceID: "instance-123",
	}

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test.jpg")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake image content"))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UploadImage(w, req)

	// handleError returns 200 with toast, verify it's not a successful JSON response
	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	if err == nil {
		assert.Empty(t, response["url"])
	}
}

func TestPlayerHandler_UploadImage_WithoutBlockID(t *testing.T) {
	// This tests that uploads can still work without block_id (for backward compatibility)
	mockService := &mockUploadService{
		returnUpload: &models.Upload{
			ID:          "upload-123",
			OriginalURL: "https://example.com/uploads/test.jpg",
			InstanceID:  "instance-123",
			TeamCode:    "TEAM1",
		},
	}

	handler := &PlayerHandler{
		logger:        slog.Default(),
		uploadService: mockService,
	}

	team := &models.Team{
		Code:       "TEAM1",
		InstanceID: "instance-123",
	}

	// Create multipart form data without block_id
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileWriter, err := writer.CreateFormFile("file", "test.jpg")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("fake image content"))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.UploadImage(w, req)

	// Should still succeed
	assert.Equal(t, http.StatusOK, w.Code)

	// But block_id should be empty
	assert.Empty(t, mockService.uploadedMetadata.BlockID)
}
