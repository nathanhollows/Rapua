package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

type OrphanedUploadsCleanupService struct {
	db         *bun.DB
	logger     *slog.Logger
	uploadsDir string
}

func NewOrphanedUploadsCleanupService(
	db *bun.DB,
	logger *slog.Logger,
	uploadsDir string,
) *OrphanedUploadsCleanupService {
	return &OrphanedUploadsCleanupService{
		db:         db,
		logger:     logger,
		uploadsDir: uploadsDir,
	}
}

// CleanupOrphanedUploads scans the uploads directory and removes files
// that are not referenced in any blocks.
func (s *OrphanedUploadsCleanupService) CleanupOrphanedUploads(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Starting orphaned uploads cleanup", "uploads_dir", s.uploadsDir)

	var filesDeleted int
	var totalFiles int

	// Walk through all files in the uploads directory
	err := filepath.Walk(s.uploadsDir, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			s.logger.Warn("Error accessing path", "path", path, "error", err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		totalFiles++

		// Convert absolute file path to URL path
		// e.g., /home/user/Rapua/static/uploads/2025/11/17/uuid.png -> /static/uploads/2025/11/17/uuid.png
		relPath, err := filepath.Rel(filepath.Dir(s.uploadsDir), path)
		if err != nil {
			s.logger.Warn("Failed to get relative path", "path", path, "error", err)
			return nil
		}

		// Convert to forward slashes for URL comparison
		urlPath := "/" + filepath.ToSlash(relPath)

		// Check if this URL is referenced in any blocks
		isReferenced, err := s.isFileReferencedInBlocks(ctx, urlPath)
		if err != nil {
			s.logger.Warn("Failed to check if file is referenced",
				"path", urlPath,
				"error", err,
			)
			return nil
		}

		if !isReferenced {
			// File is orphaned, delete it
			if removeErr := os.Remove(path); removeErr != nil {
				s.logger.Warn("Failed to delete orphaned file",
					"path", path,
					"error", removeErr,
				)
				return nil
			}

			s.logger.Info("Deleted orphaned upload",
				"path", urlPath,
				"filesystem_path", path,
			)
			filesDeleted++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk uploads directory: %w", err)
	}

	s.logger.InfoContext(ctx, "Orphaned uploads cleanup completed",
		"total_files_scanned", totalFiles,
		"files_deleted", filesDeleted,
		"files_retained", totalFiles-filesDeleted,
	)

	return nil
}

// isFileReferencedInBlocks checks if a URL path is referenced in any block's data.
// It searches through the JSON data field of all blocks for the URL.
// Only checks for local uploaded files, not external URLs.
func (s *OrphanedUploadsCleanupService) isFileReferencedInBlocks(
	ctx context.Context,
	urlPath string,
) (bool, error) {
	// Extract just the filename for search
	filename := filepath.Base(urlPath)

	// Escape LIKE special characters
	escapedFilename := escapeLikePattern(filename)

	// Get site URL for matching absolute local URLs
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8090"
	}

	// Search for the filename in blocks data, but only where it appears
	// as part of a local upload URL pattern:
	// 1. Relative path: "/static/uploads/..."
	// 2. Absolute path with site domain: "http://localhost:8090/static/uploads/..."
	// This prevents matching external URLs like https://example.com/static/uploads/file.png

	// Build two search patterns
	relativePattern := escapeLikePattern("\"/static/uploads/") + "%" + escapedFilename + "%"
	absolutePattern := escapeLikePattern("\""+siteURL+"/static/uploads/") + "%" + escapedFilename + "%"

	count, err := s.db.NewSelect().
		Model((*models.Block)(nil)).
		Where("data LIKE ? ESCAPE '\\'", "%"+relativePattern).
		WhereOr("data LIKE ? ESCAPE '\\'", "%"+absolutePattern).
		Count(ctx)

	if err != nil {
		return false, fmt.Errorf("failed to query blocks: %w", err)
	}

	return count > 0, nil
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
