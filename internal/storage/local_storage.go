package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/internal/config"
	"github.com/nathanhollows/Rapua/v8/internal/services"
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
func (s *LocalStorage) Upload(
	ctx context.Context,
	file multipart.File,
	filename string,
) (map[string]string, string, error) {
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
	err := os.MkdirAll(destPath, 0o750)
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

	deleteData := filePath
	filePath = config.SiteURL(filePath)

	// Copy the file to the destination
	_, err = io.Copy(outFile, file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to save file: %w", err)
	}

	// New filepath for the file
	return map[string]string{"original": filePath}, deleteData, nil
}

// Delete removes the file at the path Upload returned. A file already gone is
// not an error, so a retried cleanup stays safe.
func (s *LocalStorage) Delete(ctx context.Context, deleteData string) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	// The path comes back from a database row, so it must still sit inside the
	// storage root before anything is removed.
	rel, err := filepath.Rel(s.basePath, deleteData)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("refusing to delete outside storage: %q", deleteData)
	}

	err = os.Remove(deleteData)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Type returns the storage type.
func (s *LocalStorage) Type() string {
	return "local"
}
