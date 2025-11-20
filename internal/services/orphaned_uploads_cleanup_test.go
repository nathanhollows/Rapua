package services_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupOrphanedUploadsCleanupService(t *testing.T) (
	*services.OrphanedUploadsCleanupService,
	*repositories.UploadsRepository,
	*bun.DB,
	string,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	uploadRepo := repositories.NewUploadRepository(dbc)

	// Create test upload directory
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))

	service := services.NewOrphanedUploadsCleanupService(uploadRepo, newTLogger(t), uploadsDir)

	return service, &uploadRepo, dbc, uploadsDir, cleanup
}

func TestOrphanedUploadsCleanupService_CleanupOrphanedUploads(t *testing.T) {
	service, uploadRepo, dbc, uploadsDir, cleanup := setupOrphanedUploadsCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test directory structure (YYYY/MM/DD)
	testDate := "2025/11/17"
	dateDir := filepath.Join(uploadsDir, testDate)
	require.NoError(t, os.MkdirAll(dateDir, 0755))

	// Create test files
	referencedFile := filepath.Join(dateDir, "referenced-uuid.png")
	orphanedFile := filepath.Join(dateDir, "orphaned-uuid.png")

	require.NoError(t, os.WriteFile(referencedFile, []byte("test image data"), 0644))
	require.NoError(t, os.WriteFile(orphanedFile, []byte("orphaned image"), 0644))

	// Create a block for the referenced file
	block := &models.Block{
		ID:       "block-1",
		OwnerID:  "location-1",
		Type:     "image",
		Context:  "content",
		Data:     []byte(`{"content":"/static/uploads/2025/11/17/referenced-uuid.png"}`),
		Ordering: 0,
		Points:   0,
	}

	_, err := dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	// Create upload records
	referencedUpload := &models.Upload{
		ID:          "upload-1",
		OriginalURL: "/static/uploads/2025/11/17/referenced-uuid.png",
		BlockID:     "block-1", // References existing block
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	err = uploadRepo.Create(ctx, referencedUpload)
	require.NoError(t, err)

	orphanedUpload := &models.Upload{
		ID:          "upload-2",
		OriginalURL: "/static/uploads/2025/11/17/orphaned-uuid.png",
		BlockID:     "block-nonexistent", // References non-existent block
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	err = uploadRepo.Create(ctx, orphanedUpload)
	require.NoError(t, err)

	// Verify initial state
	assertFileExists(t, referencedFile)
	assertFileExists(t, orphanedFile)

	// Run cleanup
	err = service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// Verify results
	assertFileExists(t, referencedFile)  // Should still exist (block exists)
	assertFileNotExists(t, orphanedFile) // Should be deleted (block doesn't exist)

	// Verify database state
	uploads, err := uploadRepo.SearchByCriteria(ctx, map[string]string{"id": "upload-1"})
	require.NoError(t, err)
	assert.Len(t, uploads, 1, "Referenced upload should still exist in database")

	uploads, err = uploadRepo.SearchByCriteria(ctx, map[string]string{"id": "upload-2"})
	require.NoError(t, err)
	assert.Empty(t, uploads, "Orphaned upload should be deleted from database")
}

func TestOrphanedUploadsCleanupService_NoOrphanedUploads(t *testing.T) {
	service, uploadRepo, dbc, uploadsDir, cleanup := setupOrphanedUploadsCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test directory and file
	testDate := "2025/11/17"
	dateDir := filepath.Join(uploadsDir, testDate)
	require.NoError(t, os.MkdirAll(dateDir, 0755))

	referencedFile := filepath.Join(dateDir, "referenced-uuid.png")
	require.NoError(t, os.WriteFile(referencedFile, []byte("test image data"), 0644))

	// Create a block
	block := &models.Block{
		ID:       "block-2",
		OwnerID:  "location-1",
		Type:     "image",
		Context:  "content",
		Data:     []byte(`{"content":"/static/uploads/2025/11/17/referenced-uuid.png"}`),
		Ordering: 0,
		Points:   0,
	}

	_, err := dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	// Create upload record that references existing block
	upload := &models.Upload{
		ID:          "upload-3",
		OriginalURL: "/static/uploads/2025/11/17/referenced-uuid.png",
		BlockID:     "block-2",
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	err = uploadRepo.Create(ctx, upload)
	require.NoError(t, err)

	// Run cleanup
	err = service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// Verify file still exists
	assertFileExists(t, referencedFile)

	// Verify upload record still exists
	uploads, err := uploadRepo.SearchByCriteria(ctx, map[string]string{"id": "upload-3"})
	require.NoError(t, err)
	assert.Len(t, uploads, 1)
}

// TODO: Uncomment when image size variants are implemented
// func TestOrphanedUploadsCleanupService_WithImageSizes(t *testing.T) {
// 	service, uploadRepo, _, uploadsDir, cleanup := setupOrphanedUploadsCleanupService(t)
// 	defer cleanup()
//
// 	ctx := context.Background()
//
// 	// Create test directory
// 	testDate := "2025/11/17"
// 	dateDir := filepath.Join(uploadsDir, testDate)
// 	require.NoError(t, os.MkdirAll(dateDir, 0755))
//
// 	// Create main file and size variants
// 	mainFile := filepath.Join(dateDir, "orphaned-uuid.png")
// 	size1File := filepath.Join(dateDir, "orphaned-uuid-400.png")
// 	size2File := filepath.Join(dateDir, "orphaned-uuid-800.png")
//
// 	require.NoError(t, os.WriteFile(mainFile, []byte("main"), 0644))
// 	require.NoError(t, os.WriteFile(size1File, []byte("size1"), 0644))
// 	require.NoError(t, os.WriteFile(size2File, []byte("size2"), 0644))
//
// 	// Create orphaned upload with size variants
// 	orphanedUpload := &models.Upload{
// 		ID:          "upload-4",
// 		OriginalURL: "http://example.com/image.png",
// 		BlockID:     "block-nonexistent",
// 		Storage:     "/static/uploads/2025/11/17/orphaned-uuid.png",
// 		Type:        models.MediaTypeImage,
// 	}
// 	err := orphanedUpload.AddSize(400, "/static/uploads/2025/11/17/orphaned-uuid-400.png")
// 	require.NoError(t, err)
// 	err = orphanedUpload.AddSize(800, "/static/uploads/2025/11/17/orphaned-uuid-800.png")
// 	require.NoError(t, err)
//
// 	err = uploadRepo.Create(ctx, orphanedUpload)
// 	require.NoError(t, err)
//
// 	// Verify initial state
// 	assertFileExists(t, mainFile)
// 	assertFileExists(t, size1File)
// 	assertFileExists(t, size2File)
//
// 	// Run cleanup
// 	err = service.CleanupOrphanedUploads(ctx)
// 	require.NoError(t, err)
//
// 	// Verify all files are deleted
// 	assertFileNotExists(t, mainFile)
// 	assertFileNotExists(t, size1File)
// 	assertFileNotExists(t, size2File)
//
// 	// Verify upload record is deleted
// 	uploads, err := uploadRepo.SearchByCriteria(ctx, map[string]string{"id": "upload-4"})
// 	require.NoError(t, err)
// 	assert.Empty(t, uploads)
// }

func TestOrphanedUploadsCleanupService_CleanupEmptyDirectories(t *testing.T) {
	service, _, _, uploadsDir, cleanup := setupOrphanedUploadsCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create nested directory structure
	yearDir := filepath.Join(uploadsDir, "2025")
	monthDir := filepath.Join(yearDir, "11")
	dayDir := filepath.Join(monthDir, "17")
	require.NoError(t, os.MkdirAll(dayDir, 0755))

	// Verify directories exist
	_, err := os.Stat(dayDir)
	require.NoError(t, err)

	// Run cleanup on empty directories (multiple passes may be needed for nested dirs)
	err = service.CleanupEmptyDirectories(ctx)
	require.NoError(t, err)
	err = service.CleanupEmptyDirectories(ctx)
	require.NoError(t, err)
	err = service.CleanupEmptyDirectories(ctx)
	require.NoError(t, err)

	// Verify empty directories are removed
	_, err = os.Stat(dayDir)
	assert.True(t, os.IsNotExist(err), "Day directory should be deleted")

	// Root uploads directory should still exist
	_, err = os.Stat(uploadsDir)
	require.NoError(t, err, "Root uploads directory should still exist")
}

// Helper functions

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.NoError(t, err, "File should exist: %s", path)
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "File should not exist: %s", path)
}
