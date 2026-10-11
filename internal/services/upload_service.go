package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

// UploadService provides methods for uploading files and managing metadata.
type UploadService struct {
	transactor   db.Transactor
	repo         repositories.UploadsRepository
	runEventRepo repositories.RunEventRepository
	storage      UploadStorage
}

// NewUploadService creates a new UploadService.
func NewUploadService(
	transactor db.Transactor,
	repo repositories.UploadsRepository,
	runEventRepo repositories.RunEventRepository,
	storage UploadStorage,
) *UploadService {
	return &UploadService{
		transactor:   transactor,
		repo:         repo,
		runEventRepo: runEventRepo,
		storage:      storage,
	}
}

// UploadMetadata contains metadata for an uploaded file.
type UploadMetadata struct {
	QuestID string `json:"questID,omitempty"`
	RunID   string `json:"teamID,omitempty"`
	BlockID string `json:"blockID,omitempty"`
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
		QuestID:     data.QuestID,
		RunCode:     data.RunID,
		BlockID:     data.BlockID,
	}

	// The row and its event commit together, so a saved upload is always one the
	// stream knows about.
	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err = s.repo.CreateWithTx(ctx, tx, upload); err != nil {
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	// Only a run's own upload belongs in its stream: the block editor's media
	// carries no run code.
	if upload.RunCode != "" {
		event := uploadAddedEvent(
			upload.RunCode, upload.QuestID, upload.BlockID, upload.OriginalURL, string(upload.Type),
		)
		if err = s.runEventRepo.RecordWithTx(ctx, tx, event); err != nil {
			return nil, fmt.Errorf("recording upload: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing upload: %w", err)
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
