package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanhollows/Rapua/v6/repositories"
)

type OrphanedUploadsCleanupService struct {
	uploadsRepo repositories.UploadsRepository
	logger      *slog.Logger
	uploadsDir  string
}

func NewOrphanedUploadsCleanupService(
	uploadsRepo repositories.UploadsRepository,
	logger *slog.Logger,
	uploadsDir string,
) *OrphanedUploadsCleanupService {
	return &OrphanedUploadsCleanupService{
		uploadsRepo: uploadsRepo,
		logger:      logger,
		uploadsDir:  uploadsDir,
	}
}

// CleanupOrphanedUploads finds and removes upload records and files for deleted blocks.
func (s *OrphanedUploadsCleanupService) CleanupOrphanedUploads(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Starting orphaned uploads cleanup")

	// Get all orphaned uploads from the database
	orphanedUploads, err := s.uploadsRepo.GetOrphanedUploads(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch orphaned uploads: %w", err)
	}

	if len(orphanedUploads) == 0 {
		s.logger.InfoContext(ctx, "No orphaned uploads found")
		return nil
	}

	var filesDeleted int
	var recordsDeleted int

	// Delete each orphaned upload
	for _, upload := range orphanedUploads {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Delete the upload record from database
		deleteErr := s.uploadsRepo.Delete(ctx, upload.ID)
		if deleteErr != nil {
			s.logger.WarnContext(ctx, "Failed to delete upload record",
				"uploadID", upload.ID,
				"error", deleteErr,
			)
			continue
		}
		recordsDeleted++

		// Delete the main file from filesystem using OriginalURL (which contains the actual path)
		if s.deleteUploadFile(ctx, upload.OriginalURL) {
			filesDeleted++
		}

		// Delete any additional size variants
		sizes, sizeErr := upload.GetSizes()
		if sizeErr != nil {
			s.logger.WarnContext(ctx, "Failed to get upload sizes",
				"uploadID", upload.ID,
				"error", sizeErr,
			)
			continue
		}

		for _, size := range sizes {
			if s.deleteUploadFile(ctx, size.URL) {
				filesDeleted++
			}
		}
	}

	s.logger.InfoContext(ctx, "Orphaned uploads cleanup completed",
		"records_deleted", recordsDeleted,
		"files_deleted", filesDeleted,
	)

	return nil
}

// deleteUploadFile deletes a single file from the filesystem using its URL/path.
// Returns true if file was successfully deleted, false otherwise.
func (s *OrphanedUploadsCleanupService) deleteUploadFile(ctx context.Context, urlOrPath string) bool {
	// Parse the URL to get the filesystem path
	// Expected formats:
	//   - /static/uploads/YYYY/MM/DD/filename.ext (relative)
	//   - http://domain/static/uploads/YYYY/MM/DD/filename.ext (absolute)

	// Strip domain if present (convert absolute URL to path)
	path := urlOrPath
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		// Extract path from URL (everything after domain)
		//nolint:mnd // URL structure: ["http:", "", "domain", "path/to/file"]
		parts := strings.SplitN(urlOrPath, "/", 4)
		//nolint:mnd // Need 4 parts to extract path after domain
		if len(parts) >= 4 {
			path = "/" + parts[3]
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) < 6 || parts[0] != "static" || parts[1] != "uploads" {
		s.logger.WarnContext(ctx, "Unexpected upload URL format",
			"url", urlOrPath,
			"parsed_path", path)
		return false
	}

	// Extract date path: YYYY/MM/DD
	datePath := filepath.Join(parts[2], parts[3], parts[4])
	filename := filepath.Base(path)
	filePath := filepath.Join(s.uploadsDir, datePath, filename)

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		if !os.IsNotExist(err) {
			s.logger.WarnContext(ctx, "Failed to delete upload file",
				"path", filePath,
				"error", err,
			)
		}
		return false
	}

	return true
}

// CleanupEmptyDirectories removes empty directories in the uploads folder.
// This should be called after cleaning up orphaned files.
func (s *OrphanedUploadsCleanupService) CleanupEmptyDirectories(ctx context.Context) error {
	var dirsDeleted int

	// Walk the directory tree in reverse (bottom-up) to delete empty subdirectories
	err := filepath.Walk(s.uploadsDir, func(path string, info os.FileInfo, walkErr error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if walkErr != nil {
			//nolint:nilerr // Continue walking on errors, intentionally ignore per-file errors
			return nil
		}

		// Skip if not a directory or if it's the root uploads directory
		if !info.IsDir() || path == s.uploadsDir {
			return nil
		}

		// Check if directory is empty
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			s.logger.Warn("Failed to read directory", "path", path, "error", readErr)
			return nil
		}

		if len(entries) == 0 {
			if removeErr := os.Remove(path); removeErr != nil {
				s.logger.Warn("Failed to delete empty directory",
					"path", path,
					"error", removeErr,
				)
			} else {
				s.logger.Info("Deleted empty directory", "path", path)
				dirsDeleted++
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to cleanup empty directories: %w", err)
	}

	if dirsDeleted > 0 {
		s.logger.InfoContext(ctx, "Empty directories cleanup completed",
			"directories_deleted", dirsDeleted,
		)
	}

	return nil
}
