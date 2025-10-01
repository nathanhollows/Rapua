package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
)

// UploadService provides methods for uploading files and managing metadata.
type UploadService struct {
	repo    repositories.UploadsRepository
	storage UploadStorage
}

// NewUploadService creates a new UploadService.
func NewUploadService(repo repositories.UploadsRepository, storage UploadStorage) *UploadService {
	return &UploadService{
		repo:    repo,
		storage: storage,
	}
}

// UploadMetadata contains metadata for an uploaded file.
type UploadMetadata struct {
	InstanceID string `json:"instanceID,omitempty"`
	TeamID     string `json:"teamID,omitempty"`
	BlockID    string `json:"blockID,omitempty"`
	LocationID string `json:"locationID,omitempty"`
}

// UploadStorage is an interface for storing files.
type UploadStorage interface {
	Upload(ctx context.Context, file multipart.File, filename string) (map[string]string, string, error)
	Type() string
}

// UploadFile uploads a file and saves metadata to the database.
func (s *UploadService) UploadFile(
	ctx context.Context,
	file multipart.File,
	fileHeader *multipart.FileHeader,
	data UploadMetadata,
) (*models.Upload, error) {
	if fileHeader == nil {
		return nil, errors.New("file header is nil")
	}

	// Validate file type
	var fileType models.MediaType
	allowedMimeTypes := map[string]models.MediaType{
		// Images
		"image/jpeg": models.MediaTypeImage,
		"image/png":  models.MediaTypeImage,
		"image/gif":  models.MediaTypeImage,
		"image/webp": models.MediaTypeImage,
		// Videos
		"video/mp4":       models.MediaTypeVideo,
		"video/quicktime": models.MediaTypeVideo,
		"video/webm":      models.MediaTypeVideo,
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if val, ok := allowedMimeTypes[contentType]; ok {
		fileType = val
	} else {
		return nil, fmt.Errorf("unsupported file type: %s", contentType)
	}

	// Upload file to storage (local or S3)
	links, deleteData, err := s.storage.Upload(ctx, file, fileHeader.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	originalURL, ok := links["original"]
	if !ok {
		return nil, errors.New("storage did not return original URL")
	}

	upload := &models.Upload{
		OriginalURL: originalURL,
		Timestamp:   time.Now(),
		Storage:     s.storage.Type(),
		DeleteData:  deleteData,
		Type:        fileType,
		InstanceID:  data.InstanceID,
		TeamCode:    data.TeamID,
		BlockID:     data.BlockID,
		LocationID:  data.LocationID,
	}

	// Save metadata to database
	if err := s.repo.Create(ctx, upload); err != nil {
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	return upload, nil
}

// Search retrieves uploads based on search criteria.
func (s *UploadService) Search(ctx context.Context, filters map[string]string) ([]*models.Upload, error) {
	if len(filters) == 0 {
		return nil, errors.New("at least one filter is required")
	}
	return s.repo.SearchByCriteria(ctx, filters)
}
