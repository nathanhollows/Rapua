package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/helpers"
	"github.com/nathanhollows/Rapua/v4/internal/services"
)

// LocalStorage is a storage implementation that saves files to the local filesystem.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local storage instance.
func NewLocalStorage(basePath string) services.UploadStorage {
	return &LocalStorage{basePath: basePath}
}

// Upload saves the file to the local storage.
func (s *LocalStorage) Upload(ctx context.Context, file multipart.File, filename string) (map[string]string, string, error) {
	select {
	case <-ctx.Done():
		return nil, "", fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	// Generate a unique ID for the file
	id := uuid.New().String()
	date := time.Now().Format("2006/01/02")

	// Clean the filename
	filename = filepath.Clean(filepath.Base(filename))

	// Create the directory for the file
	destPath := filepath.Join(s.basePath, date)
	err := os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Check the file does not exist
	filePath := filepath.Join(destPath, id+filepath.Ext(filename))
	_, err = os.Stat(filePath)
	if err == nil {
		return nil, "", fmt.Errorf("file already exists: %s", filePath)
	}

	// Save the file
	outFile, err := os.Create(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	filePath = helpers.URL(filePath)

	// Copy the file to the destination
	_, err = io.Copy(outFile, file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to save file: %w", err)
	}

	// New filepath for the file
	return map[string]string{"original": filePath}, "", nil
}

// Type returns the storage type.
func (s *LocalStorage) Type() string {
	return "local"
}
