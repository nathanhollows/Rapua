package repositories_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUploadsRepository(t *testing.T) (repositories.UploadsRepository, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	uploadsRepository := repositories.NewUploadRepository(dbc)

	return uploadsRepository, cleanup
}

func TestUploadRepository_Create(t *testing.T) {
	repo, cleanup := setupUploadsRepository(t)
	defer cleanup()

	tests := []struct {
		name      string
		upload    *models.Upload
		expectErr bool
	}{
		{
			name:      "Valid Upload",
			upload:    &models.Upload{OriginalURL: "https://cdn.example.com/original.jpg"},
			expectErr: false,
		},
		{
			name:      "Nil Upload",
			upload:    nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), tt.upload)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, tt.upload.ID) // Ensure ID was generated
			}
		})
	}
}

func TestUploadRepository_SearchByCriteria(t *testing.T) {
	repo, cleanup := setupUploadsRepository(t)
	defer cleanup()

	tests := []struct {
		name      string
		criteria  map[string]string
		expectErr bool
	}{
		{
			name:      "Valid Search by ID",
			criteria:  map[string]string{"id": uuid.New().String()},
			expectErr: false,
		},
		{
			name:      "Invalid Field",
			criteria:  map[string]string{"invalid_field": "value"},
			expectErr: true,
		},
		{
			name:      "Empty Criteria",
			criteria:  map[string]string{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.SearchByCriteria(context.Background(), tt.criteria)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUploadRepository_GetByBlockID(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewUploadRepository(dbc)
	ctx := context.Background()

	// Create real blocks to avoid orphaned uploads
	block1 := &models.Block{
		ID:       "block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	block2 := &models.Block{
		ID:       "different-block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	_, err := dbc.NewInsert().Model(block1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(block2).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads with the block IDs
	upload1 := &models.Upload{
		OriginalURL: "https://example.com/image1.jpg",
		BlockID:     block1.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	upload2 := &models.Upload{
		OriginalURL: "https://example.com/image2.jpg",
		BlockID:     block1.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	upload3 := &models.Upload{
		OriginalURL: "https://example.com/image3.jpg",
		BlockID:     block2.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}

	require.NoError(t, repo.Create(ctx, upload1))
	require.NoError(t, repo.Create(ctx, upload2))
	require.NoError(t, repo.Create(ctx, upload3))

	// Get uploads for the specific block
	uploads, err := repo.GetByBlockID(ctx, block1.ID)
	require.NoError(t, err)

	// Should return exactly 2 uploads
	assert.Len(t, uploads, 2)

	// Verify the correct uploads were returned
	uploadIDs := make(map[string]bool)
	for _, upload := range uploads {
		uploadIDs[upload.ID] = true
		assert.Equal(t, block1.ID, upload.BlockID)
	}
	assert.True(t, uploadIDs[upload1.ID])
	assert.True(t, uploadIDs[upload2.ID])
	assert.False(t, uploadIDs[upload3.ID])
}

func TestUploadRepository_GetByBlockID_NoResults(t *testing.T) {
	repo, cleanup := setupUploadsRepository(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentBlockID := "nonexistent-" + uuid.New().String()

	uploads, err := repo.GetByBlockID(ctx, nonExistentBlockID)
	require.NoError(t, err)
	assert.Empty(t, uploads)
}

func TestUploadRepository_Delete(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewUploadRepository(dbc)
	ctx := context.Background()

	// Create a real block to avoid orphaned uploads
	block := &models.Block{
		ID:       "block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	_, err := dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	// Create a test upload
	upload := &models.Upload{
		OriginalURL: "https://example.com/image.jpg",
		BlockID:     block.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	require.NoError(t, repo.Create(ctx, upload))

	// Verify upload exists
	uploads, err := repo.SearchByCriteria(ctx, map[string]string{"id": upload.ID})
	require.NoError(t, err)
	assert.Len(t, uploads, 1)

	// Delete the upload
	err = repo.Delete(ctx, upload.ID)
	require.NoError(t, err)

	// Verify upload no longer exists
	uploads, err = repo.SearchByCriteria(ctx, map[string]string{"id": upload.ID})
	require.NoError(t, err)
	assert.Empty(t, uploads)
}

func TestUploadRepository_Delete_NonExistent(t *testing.T) {
	repo, cleanup := setupUploadsRepository(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentID := "nonexistent-" + uuid.New().String()

	// Deleting non-existent upload should not error
	err := repo.Delete(ctx, nonExistentID)
	require.NoError(t, err)
}

func TestUploadRepository_DeleteByBlockID(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewUploadRepository(dbc)
	ctx := context.Background()

	// Create real blocks to avoid orphaned uploads
	block1 := &models.Block{
		ID:       "block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	block2 := &models.Block{
		ID:       "other-block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	_, err := dbc.NewInsert().Model(block1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(block2).Exec(ctx)
	require.NoError(t, err)

	// Create multiple uploads for the same block
	upload1 := &models.Upload{
		OriginalURL: "https://example.com/image1.jpg",
		BlockID:     block1.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	upload2 := &models.Upload{
		OriginalURL: "https://example.com/image2.jpg",
		BlockID:     block1.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	upload3 := &models.Upload{
		OriginalURL: "https://example.com/image3.jpg",
		BlockID:     block2.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}

	require.NoError(t, repo.Create(ctx, upload1))
	require.NoError(t, repo.Create(ctx, upload2))
	require.NoError(t, repo.Create(ctx, upload3))

	// Delete uploads for the specific block
	err = repo.DeleteByBlockID(ctx, block1.ID)
	require.NoError(t, err)

	// Verify uploads for the block were deleted
	uploads, err := repo.GetByBlockID(ctx, block1.ID)
	require.NoError(t, err)
	assert.Empty(t, uploads)

	// Verify upload for other block still exists
	uploads, err = repo.GetByBlockID(ctx, block2.ID)
	require.NoError(t, err)
	assert.Len(t, uploads, 1)
	assert.Equal(t, upload3.ID, uploads[0].ID)
}

func TestUploadRepository_DeleteByBlockID_NoResults(t *testing.T) {
	repo, cleanup := setupUploadsRepository(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentBlockID := "nonexistent-" + uuid.New().String()

	// Deleting uploads for non-existent block should not error
	err := repo.DeleteByBlockID(ctx, nonExistentBlockID)
	require.NoError(t, err)
}

func TestUploadRepository_GetOrphanedUploads(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewUploadRepository(dbc)
	ctx := context.Background()

	// Create a real block for testing (using direct DB insert)
	block := &models.Block{
		ID:       "block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	_, err := dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	// Create uploads:
	// 1. Upload with existing block (not orphaned)
	upload1 := &models.Upload{
		OriginalURL: "https://example.com/image1.jpg",
		BlockID:     block.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	// 2. Upload with non-existent block (orphaned)
	upload2 := &models.Upload{
		OriginalURL: "https://example.com/image2.jpg",
		BlockID:     "nonexistent-block-" + uuid.New().String(),
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	// 3. Upload with null block_id (not orphaned, just unassigned)
	upload3 := &models.Upload{
		OriginalURL: "https://example.com/image3.jpg",
		BlockID:     "",
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}

	require.NoError(t, repo.Create(ctx, upload1))
	require.NoError(t, repo.Create(ctx, upload2))
	require.NoError(t, repo.Create(ctx, upload3))

	// Get orphaned uploads
	orphaned, err := repo.GetOrphanedUploads(ctx)
	require.NoError(t, err)

	// Check that our orphaned upload is in the results
	foundOrphan := false
	for _, upload := range orphaned {
		if upload.ID == upload2.ID {
			foundOrphan = true
			break
		}
	}
	assert.True(t, foundOrphan, "Should find upload2 as orphaned")

	// Verify upload1 (with valid block) is NOT in orphaned list
	for _, upload := range orphaned {
		assert.NotEqual(t, upload1.ID, upload.ID, "Upload with valid block should not be orphaned")
	}

	// Verify upload3 (with null block_id) is NOT in orphaned list
	for _, upload := range orphaned {
		assert.NotEqual(t, upload3.ID, upload.ID, "Upload with null block_id should not be orphaned")
	}
}

func TestUploadRepository_GetOrphanedUploads_NoOrphans(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewUploadRepository(dbc)
	ctx := context.Background()

	// Create a real block (using direct DB insert)
	block := &models.Block{
		ID:       "block-" + uuid.New().String(),
		OwnerID:  "location-" + uuid.New().String(),
		Type:     "text",
		Context:  "content",
		Data:     []byte(`{"content": "test"}`),
		Ordering: 0,
		Points:   0,
	}
	_, err := dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	// Create upload that references the real block
	upload := &models.Upload{
		OriginalURL: "https://example.com/image.jpg",
		BlockID:     block.ID,
		Storage:     "local",
		Type:        models.MediaTypeImage,
	}
	require.NoError(t, repo.Create(ctx, upload))

	// Get orphaned uploads
	orphaned, err := repo.GetOrphanedUploads(ctx)
	require.NoError(t, err)

	// Verify our upload (with valid block) is NOT in the orphaned list
	for _, orphanedUpload := range orphaned {
		assert.NotEqual(t, upload.ID, orphanedUpload.ID,
			"Upload with valid block reference should not be in orphaned list")
	}
}
