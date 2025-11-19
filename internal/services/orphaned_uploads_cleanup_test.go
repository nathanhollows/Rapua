package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrphanedUploadsCleanupService_CleanupOrphanedUploads(t *testing.T) {
	// Setup test database
	db, cleanup := setupDB(t)
	defer cleanup()

	// Create test upload directory
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))

	// Create test directory structure (YYYY/MM/DD)
	testDate := "2025/11/17"
	dateDir := filepath.Join(uploadsDir, testDate)
	require.NoError(t, os.MkdirAll(dateDir, 0755))

	// Create test files
	referencedFile := filepath.Join(dateDir, "referenced-uuid.png")
	orphanedFile := filepath.Join(dateDir, "orphaned-uuid.png")
	anotherOrphanedFile := filepath.Join(dateDir, "another-orphaned-uuid.jpg")

	require.NoError(t, os.WriteFile(referencedFile, []byte("test image data"), 0644))
	require.NoError(t, os.WriteFile(orphanedFile, []byte("orphaned image"), 0644))
	require.NoError(t, os.WriteFile(anotherOrphanedFile, []byte("another orphaned"), 0644))

	// Create a block that references the first file
	ctx := context.Background()
	imageBlock := blocks.NewImageBlock(blocks.BaseBlock{
		ID:         "block-1",
		LocationID: "location-1",
		Type:       "image",
		Order:      0,
		Points:     0,
	})
	imageBlock.URL = "/static/uploads/" + testDate + "/referenced-uuid.png"

	modelBlock := models.Block{
		ID:       imageBlock.ID,
		OwnerID:  imageBlock.LocationID,
		Type:     imageBlock.GetType(),
		Context:  blocks.ContextLocationContent,
		Data:     imageBlock.GetData(),
		Ordering: 0,
		Points:   0,
	}
	_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
	require.NoError(t, err)

	// Verify initial state
	assertFileExists(t, referencedFile)
	assertFileExists(t, orphanedFile)
	assertFileExists(t, anotherOrphanedFile)

	// Create service and run cleanup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, uploadsDir)

	err = service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// Verify results
	assertFileExists(t, referencedFile)         // Should still exist
	assertFileNotExists(t, orphanedFile)        // Should be deleted
	assertFileNotExists(t, anotherOrphanedFile) // Should be deleted
}

func TestOrphanedUploadsCleanupService_FileReferencedInBlocks(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create temp dir for service
	tempDir := t.TempDir()

	t.Run("file referenced in image block", func(t *testing.T) {
		// Create an image block with a file reference
		imageBlock := blocks.NewImageBlock(blocks.BaseBlock{
			ID:         "block-referenced",
			LocationID: "location-1",
			Type:       "image",
			Order:      0,
			Points:     0,
		})
		imageBlock.URL = "/static/uploads/2025/11/17/my-image.png"

		modelBlock := models.Block{
			ID:       imageBlock.ID,
			OwnerID:  imageBlock.LocationID,
			Type:     imageBlock.GetType(),
			Context:  blocks.ContextLocationContent,
			Data:     imageBlock.GetData(),
			Ordering: 0,
			Points:   0,
		}
		_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
		require.NoError(t, err)

		// The service has an unexported method isFileReferencedInBlocks
		// We'll test this indirectly through the main CleanupOrphanedUploads method
		// by checking if files are deleted or not
	})

	t.Run("handles different URL formats", func(t *testing.T) {
		// Create blocks with both local and external URLs with same filename
		localImageBlock := blocks.NewImageBlock(blocks.BaseBlock{
			ID:         "block-local-format",
			LocationID: "location-2",
			Type:       "image",
			Order:      0,
			Points:     0,
		})
		localImageBlock.URL = "/static/uploads/2025/11/17/format-test.png"

		externalImageBlock := blocks.NewImageBlock(blocks.BaseBlock{
			ID:         "block-external-format",
			LocationID: "location-2",
			Type:       "image",
			Order:      1,
			Points:     0,
		})
		externalImageBlock.URL = "https://example.com/static/uploads/2025/11/17/external-format-test.png"

		// Insert both blocks
		for _, block := range []*blocks.ImageBlock{localImageBlock, externalImageBlock} {
			modelBlock := models.Block{
				ID:       block.ID,
				OwnerID:  block.LocationID,
				Type:     block.GetType(),
				Context:  blocks.ContextLocationContent,
				Data:     block.GetData(),
				Ordering: block.Order,
				Points:   0,
			}
			_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
			require.NoError(t, err)
		}

		// Create the actual files
		uploadsDir := filepath.Join(tempDir, "static", "uploads", "2025", "11", "17")
		require.NoError(t, os.MkdirAll(uploadsDir, 0755))

		localFile := filepath.Join(uploadsDir, "format-test.png")
		externalFile := filepath.Join(uploadsDir, "external-format-test.png")

		require.NoError(t, os.WriteFile(localFile, []byte("local"), 0644))
		require.NoError(t, os.WriteFile(externalFile, []byte("external"), 0644))

		// Run cleanup
		service := services.NewOrphanedUploadsCleanupService(db, logger, filepath.Join(tempDir, "static", "uploads"))
		err := service.CleanupOrphanedUploads(ctx)
		require.NoError(t, err)

		// Local file should be kept (referenced by local block)
		assertFileExists(t, localFile)

		// External file should be deleted (only referenced by external URL)
		assertFileNotExists(t, externalFile)
	})
}

func TestOrphanedUploadsCleanupService_CleanupEmptyDirectories(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	// Create test directory structure
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads")

	emptyDir1 := filepath.Join(uploadsDir, "2025", "01", "15")
	emptyDir2 := filepath.Join(uploadsDir, "2025", "02", "20")
	nonEmptyDir := filepath.Join(uploadsDir, "2025", "03", "25")

	require.NoError(t, os.MkdirAll(emptyDir1, 0755))
	require.NoError(t, os.MkdirAll(emptyDir2, 0755))
	require.NoError(t, os.MkdirAll(nonEmptyDir, 0755))

	// Add a file to nonEmptyDir
	testFile := filepath.Join(nonEmptyDir, "test.png")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Verify initial state
	assertDirExists(t, emptyDir1)
	assertDirExists(t, emptyDir2)
	assertDirExists(t, nonEmptyDir)

	// Create service and cleanup empty directories
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, uploadsDir)

	ctx := context.Background()
	err := service.CleanupEmptyDirectories(ctx)
	require.NoError(t, err)

	// Verify empty directories were deleted
	assertDirNotExists(t, emptyDir1)
	assertDirNotExists(t, emptyDir2)

	// Verify non-empty directory still exists
	assertDirExists(t, nonEmptyDir)
	assertFileExists(t, testFile)
}

func TestOrphanedUploadsCleanupService_MultipleBlocks(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple blocks referencing the same file
	filename := "shared-image.png"
	url := "/static/uploads/2025/11/17/" + filename

	for i := range 3 {
		imageBlock := blocks.NewImageBlock(blocks.BaseBlock{
			ID:         fmt.Sprintf("block-multi-%d", i),
			LocationID: "location-1",
			Type:       "image",
			Order:      i,
			Points:     0,
		})
		imageBlock.URL = url

		modelBlock := models.Block{
			ID:       imageBlock.ID,
			OwnerID:  imageBlock.LocationID,
			Type:     imageBlock.GetType(),
			Context:  blocks.ContextLocationContent,
			Data:     imageBlock.GetData(),
			Ordering: i,
			Points:   0,
		}
		_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
		require.NoError(t, err)
	}

	// Create the file
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads", "2025", "11", "17")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))
	sharedFile := filepath.Join(uploadsDir, filename)
	require.NoError(t, os.WriteFile(sharedFile, []byte("shared"), 0644))

	// Run cleanup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, filepath.Join(tempDir, "static", "uploads"))

	err := service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// File should still exist because it's referenced by multiple blocks
	assertFileExists(t, sharedFile)
}

func TestOrphanedUploadsCleanupService_CaseInsensitiveExtensions(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	// Create test upload directory
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads", "2025", "11", "17")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))

	// Create files with different case extensions
	pngFile := filepath.Join(uploadsDir, "image.png")
	jpgFile := filepath.Join(uploadsDir, "photo.JPG")
	jpegFile := filepath.Join(uploadsDir, "picture.jpeg")

	require.NoError(t, os.WriteFile(pngFile, []byte("png"), 0644))
	require.NoError(t, os.WriteFile(jpgFile, []byte("jpg"), 0644))
	require.NoError(t, os.WriteFile(jpegFile, []byte("jpeg"), 0644))

	// Create blocks referencing only the PNG file
	ctx := context.Background()
	imageBlock := blocks.NewImageBlock(blocks.BaseBlock{
		ID:         "block-png",
		LocationID: "location-1",
		Type:       "image",
		Order:      0,
		Points:     0,
	})
	imageBlock.URL = "/static/uploads/2025/11/17/image.png"

	modelBlock := models.Block{
		ID:       imageBlock.ID,
		OwnerID:  imageBlock.LocationID,
		Type:     imageBlock.GetType(),
		Context:  blocks.ContextLocationContent,
		Data:     imageBlock.GetData(),
		Ordering: 0,
		Points:   0,
	}
	_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
	require.NoError(t, err)

	// Run cleanup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, filepath.Join(tempDir, "static", "uploads"))

	err = service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// Verify results
	assertFileExists(t, pngFile)     // Referenced, should exist
	assertFileNotExists(t, jpgFile)  // Not referenced, should be deleted
	assertFileNotExists(t, jpegFile) // Not referenced, should be deleted
}

func TestOrphanedUploadsCleanupService_EmptyDirectory(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	// Create empty upload directory
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))

	// Run cleanup on empty directory
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, uploadsDir)

	ctx := context.Background()
	err := service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err, "Cleanup should handle empty directory gracefully")
}

func TestOrphanedUploadsCleanupService_SQLiteJSONQuery(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a block with JSON data containing a filename
	imageBlock := blocks.NewImageBlock(blocks.BaseBlock{
		ID:         "block-json-test",
		LocationID: "location-1",
		Type:       "image",
		Order:      0,
		Points:     0,
	})
	imageBlock.URL = "/static/uploads/2025/11/17/test-file.png"
	imageBlock.Caption = "Test caption"

	modelBlock := models.Block{
		ID:       imageBlock.ID,
		OwnerID:  imageBlock.LocationID,
		Type:     imageBlock.GetType(),
		Context:  blocks.ContextLocationContent,
		Data:     imageBlock.GetData(),
		Ordering: 0,
		Points:   0,
	}
	_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
	require.NoError(t, err)

	// Verify the data field contains the filename (SQLite stores JSON as TEXT)
	var retrievedBlock models.Block
	err = db.NewSelect().Model(&retrievedBlock).Where("id = ?", "block-json-test").Scan(ctx)
	require.NoError(t, err)

	// Check that the JSON data contains the filename
	dataStr := string(retrievedBlock.Data)
	assert.Contains(t, dataStr, "test-file.png", "Block data should contain the filename")

	// Test the LIKE query works in SQLite (no ::text casting needed)
	count, err := db.NewSelect().
		Model((*models.Block)(nil)).
		Where("data LIKE ?", "%test-file.png%").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should find the block with LIKE query on JSON data")
}

func TestOrphanedUploadsCleanupService_ExternalURLsNotCleaned(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create blocks with external URLs
	externalBlock := blocks.NewImageBlock(blocks.BaseBlock{
		ID:         "block-external-not-" + t.Name(),
		LocationID: "location-1",
		Type:       "image",
		Order:      0,
		Points:     0,
	})
	externalBlock.URL = "https://example.com/external-image.png"

	modelBlock := models.Block{
		ID:       externalBlock.ID,
		OwnerID:  externalBlock.LocationID,
		Type:     externalBlock.GetType(),
		Context:  blocks.ContextLocationContent,
		Data:     externalBlock.GetData(),
		Ordering: 0,
		Points:   0,
	}
	_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
	require.NoError(t, err)

	// Create a local file with the same name as the external URL
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads", "2025", "11", "18")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))
	localFile := filepath.Join(uploadsDir, "external-image.png")
	require.NoError(t, os.WriteFile(localFile, []byte("local file"), 0644))

	// Run cleanup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, filepath.Join(tempDir, "static", "uploads"))

	err = service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// Local file should be deleted because the block references an external URL,
	// not the local upload path
	assertFileNotExists(t, localFile)
}

func TestOrphanedUploadsCleanupService_OnlyLocalUploadsChecked(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create blocks with mix of local and external URLs
	localBlock := blocks.NewImageBlock(blocks.BaseBlock{
		ID:         "block-local-only-" + t.Name(),
		LocationID: "location-1",
		Type:       "image",
		Order:      0,
		Points:     0,
	})
	localBlock.URL = "/static/uploads/2025/11/18/local-image.png"

	externalBlock := blocks.NewImageBlock(blocks.BaseBlock{
		ID:         "block-external-only-" + t.Name(),
		LocationID: "location-1",
		Type:       "image",
		Order:      1,
		Points:     0,
	})
	externalBlock.URL = "https://cdn.example.com/static/uploads/2025/11/18/same-filename.png"

	// Insert both blocks
	for _, block := range []*blocks.ImageBlock{localBlock, externalBlock} {
		modelBlock := models.Block{
			ID:       block.ID,
			OwnerID:  block.LocationID,
			Type:     block.GetType(),
			Context:  blocks.ContextLocationContent,
			Data:     block.GetData(),
			Ordering: block.Order,
			Points:   0,
		}
		_, err := db.NewInsert().Model(&modelBlock).Exec(ctx)
		require.NoError(t, err)
	}

	// Create files
	tempDir := t.TempDir()
	uploadsDir := filepath.Join(tempDir, "static", "uploads", "2025", "11", "18")
	require.NoError(t, os.MkdirAll(uploadsDir, 0755))

	localFile := filepath.Join(uploadsDir, "local-image.png")
	externalFile := filepath.Join(uploadsDir, "same-filename.png")

	require.NoError(t, os.WriteFile(localFile, []byte("local"), 0644))
	require.NoError(t, os.WriteFile(externalFile, []byte("external"), 0644))

	// Run cleanup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := services.NewOrphanedUploadsCleanupService(db, logger, filepath.Join(tempDir, "static", "uploads"))

	err := service.CleanupOrphanedUploads(ctx)
	require.NoError(t, err)

	// Local file should exist (referenced by local block)
	assertFileExists(t, localFile)

	// External filename file should be deleted (only referenced by external URL)
	assertFileNotExists(t, externalFile)
}

// Helper functions

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.NoError(t, err, "File should exist: %s", path)
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "File should not exist: %s", path)
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err, "Directory should exist: %s", path)
	require.True(t, info.IsDir(), "Path should be a directory: %s", path)
}

func assertDirNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "Directory should not exist: %s", path)
}
