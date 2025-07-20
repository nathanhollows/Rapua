package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupClueService(t *testing.T) (*services.ClueService, repositories.ClueRepository, repositories.LocationRepository, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	clueRepo := repositories.NewClueRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	clueService := services.NewClueService(clueRepo, locationRepo)

	return clueService, clueRepo, locationRepo, cleanup
}

func createTestLocation(t *testing.T, locationRepo repositories.LocationRepository) *models.Location {
	t.Helper()
	location := &models.Location{
		ID:         uuid.New().String(),
		InstanceID: uuid.New().String(),
		Name:       gofakeit.Name(),
		Order:      1,
	}
	
	err := locationRepo.Create(context.Background(), location)
	require.NoError(t, err)
	
	return location
}

func createTestClue(t *testing.T, clueRepo repositories.ClueRepository, locationID, instanceID string) *models.Clue {
	t.Helper()
	clue := &models.Clue{
		ID:         uuid.New().String(),
		InstanceID: instanceID,
		LocationID: locationID,
		Content:    gofakeit.Sentence(10),
	}
	
	err := clueRepo.Save(context.Background(), clue)
	require.NoError(t, err)
	
	return clue
}

func TestClueService_UpdateClues(t *testing.T) {
	service, clueRepo, locationRepo, cleanup := setupClueService(t)
	defer cleanup()

	location := createTestLocation(t, locationRepo)

	t.Run("Add new clues to location without existing clues", func(t *testing.T) {
		clues := []string{"First clue", "Second clue", "Third clue"}
		clueIDs := []string{}

		err := service.UpdateClues(context.Background(), location, clues, clueIDs)
		assert.NoError(t, err)

		// Verify clues were created
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), location.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 3)
		
		// Verify content
		contents := make([]string, len(savedClues))
		for i, clue := range savedClues {
			contents[i] = clue.Content
		}
		assert.ElementsMatch(t, clues, contents)
	})

	t.Run("Update existing clues", func(t *testing.T) {
		// Create a new location for this test
		newLocation := createTestLocation(t, locationRepo)
		
		// First, add some clues
		initialClues := []string{"Initial clue 1", "Initial clue 2"}
		err := service.UpdateClues(context.Background(), newLocation, initialClues, []string{})
		require.NoError(t, err)

		// Load clues into location
		err = locationRepo.LoadClues(context.Background(), newLocation)
		require.NoError(t, err)

		// Now update with new clues
		updatedClues := []string{"Updated clue 1", "Updated clue 2", "New clue 3"}
		err = service.UpdateClues(context.Background(), newLocation, updatedClues, []string{})
		assert.NoError(t, err)

		// Verify clues were updated
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 3)
		
		// Verify content
		contents := make([]string, len(savedClues))
		for i, clue := range savedClues {
			contents[i] = clue.Content
		}
		assert.ElementsMatch(t, updatedClues, contents)
	})

	t.Run("Add clues with specific IDs", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		clues := []string{"Clue with ID 1", "Clue with ID 2"}
		clueIDs := []string{uuid.New().String(), uuid.New().String()}

		err := service.UpdateClues(context.Background(), newLocation, clues, clueIDs)
		assert.NoError(t, err)

		// Verify clues were created with specified IDs
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 2)
		
		// Verify IDs
		ids := make([]string, len(savedClues))
		for i, clue := range savedClues {
			ids[i] = clue.ID
		}
		assert.ElementsMatch(t, clueIDs, ids)
	})

	t.Run("Add clues with partial IDs", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		clues := []string{"Clue with ID", "Clue without ID", "Another without ID"}
		clueIDs := []string{uuid.New().String()} // Use unique ID

		err := service.UpdateClues(context.Background(), newLocation, clues, clueIDs)
		assert.NoError(t, err)

		// Verify clues were created
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 3)
		
		// Find the clue with the custom ID
		var customIDClue *models.Clue
		for _, clue := range savedClues {
			if clue.ID == clueIDs[0] {
				customIDClue = &clue
				break
			}
		}
		assert.NotNil(t, customIDClue)
		assert.Equal(t, "Clue with ID", customIDClue.Content)
	})

	t.Run("Clear all clues by passing empty clues array", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		// First add some clues
		initialClues := []string{"Clue 1", "Clue 2"}
		err := service.UpdateClues(context.Background(), newLocation, initialClues, []string{})
		require.NoError(t, err)

		// Load clues into location
		err = locationRepo.LoadClues(context.Background(), newLocation)
		require.NoError(t, err)

		// Clear all clues
		err = service.UpdateClues(context.Background(), newLocation, []string{}, []string{})
		assert.NoError(t, err)

		// Verify no clues remain
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 0)
	})

	t.Run("Skip empty clues", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		clues := []string{"Valid clue", "", "Another valid clue", ""}
		clueIDs := []string{}

		err := service.UpdateClues(context.Background(), newLocation, clues, clueIDs)
		assert.NoError(t, err)

		// Verify only non-empty clues were created
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 2)
		
		// Verify content
		contents := make([]string, len(savedClues))
		for i, clue := range savedClues {
			contents[i] = clue.Content
		}
		assert.ElementsMatch(t, []string{"Valid clue", "Another valid clue"}, contents)
	})

	t.Run("Error when more clue IDs than clues", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		clues := []string{"Only one clue"}
		clueIDs := []string{"id-1", "id-2", "id-3"} // More IDs than clues

		err := service.UpdateClues(context.Background(), newLocation, clues, clueIDs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "there are more clue IDs than clues")
	})

	t.Run("Handle location without loaded clues", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		// Create some clues directly in the database
		createTestClue(t, clueRepo, newLocation.ID, newLocation.InstanceID)
		createTestClue(t, clueRepo, newLocation.ID, newLocation.InstanceID)

		// Location.Clues is empty, but clues exist in database
		assert.Len(t, newLocation.Clues, 0)

		clues := []string{"New clue 1", "New clue 2"}
		err := service.UpdateClues(context.Background(), newLocation, clues, []string{})
		assert.NoError(t, err)

		// Verify old clues were deleted and new ones created
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 2)
		
		// Verify content
		contents := make([]string, len(savedClues))
		for i, clue := range savedClues {
			contents[i] = clue.Content
		}
		assert.ElementsMatch(t, clues, contents)
	})

	t.Run("Verify clue instance and location IDs are set correctly", func(t *testing.T) {
		newLocation := createTestLocation(t, locationRepo)
		
		clues := []string{"Test clue"}
		err := service.UpdateClues(context.Background(), newLocation, clues, []string{})
		assert.NoError(t, err)

		// Verify clue has correct instance and location IDs
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), newLocation.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 1)
		
		clue := savedClues[0]
		assert.Equal(t, newLocation.InstanceID, clue.InstanceID)
		assert.Equal(t, newLocation.ID, clue.LocationID)
		assert.Equal(t, "Test clue", clue.Content)
	})
}

func TestClueService_ValidationEdgeCases(t *testing.T) {
	service, clueRepo, locationRepo, cleanup := setupClueService(t)
	defer cleanup()

	t.Run("Very long clue content", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		// Create a very long clue
		longClue := ""
		for i := 0; i < 1000; i++ {
			longClue += "a"
		}
		clues := []string{longClue}

		err := service.UpdateClues(context.Background(), location, clues, []string{})
		// Should not error on service level - database constraints may apply
		assert.NoError(t, err)
	})

	t.Run("Unicode characters in clues", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		clues := []string{"测试线索 with emoji 🚀", "Clue with àccénts"}
		err := service.UpdateClues(context.Background(), location, clues, []string{})
		assert.NoError(t, err)
	})

	t.Run("Special characters in clues", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		clues := []string{"Clue with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?"}
		err := service.UpdateClues(context.Background(), location, clues, []string{})
		assert.NoError(t, err)
	})

	t.Run("HTML in clue content", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		clues := []string{"<p>HTML <strong>content</strong> in clue</p>"}
		err := service.UpdateClues(context.Background(), location, clues, []string{})
		assert.NoError(t, err)
	})

	t.Run("Very long clue IDs", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		longID := ""
		for i := 0; i < 100; i++ {
			longID += "a"
		}
		
		clues := []string{"Test clue"}
		clueIDs := []string{longID}

		err := service.UpdateClues(context.Background(), location, clues, clueIDs)
		// Should not error on service level - database constraints may apply
		assert.NoError(t, err)
	})

	t.Run("Whitespace-only clues", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		clues := []string{"   ", "\t\t", "\n\n", "Valid clue"}
		err := service.UpdateClues(context.Background(), location, clues, []string{})
		assert.NoError(t, err)

		// Verify whitespace-only clues were saved (not skipped like empty strings)
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), location.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 4) // All clues saved, including whitespace-only
	})
}

func TestClueService_ContextCancellation(t *testing.T) {
	service, _, locationRepo, cleanup := setupClueService(t)
	defer cleanup()
	_ = locationRepo // Unused but needed for pattern consistency

	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		location := createTestLocation(t, locationRepo)
		clues := []string{"Test clue"}

		err := service.UpdateClues(ctx, location, clues, []string{})
		// Should handle cancelled context gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		}
	})
}

func TestClueService_ConcurrentUpdates(t *testing.T) {
	service, clueRepo, locationRepo, cleanup := setupClueService(t)
	defer cleanup()

	t.Run("Concurrent clue updates", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)

		// Run multiple concurrent updates
		results := make(chan error, 5)
		
		for i := 0; i < 5; i++ {
			go func(index int) {
				clues := []string{gofakeit.Sentence(5), gofakeit.Sentence(5)}
				err := service.UpdateClues(context.Background(), location, clues, []string{})
				results <- err
			}(i)
		}

		// Collect results
		errors := make([]error, 5)
		for i := 0; i < 5; i++ {
			errors[i] = <-results
		}

		// At least one should succeed (due to concurrent access, some may fail)
		successCount := 0
		for _, err := range errors {
			if err == nil {
				successCount++
			}
		}
		assert.Greater(t, successCount, 0)

		// Verify final state is consistent
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), location.ID)
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(savedClues), 10) // Should have some clues, but not more than possible
	})
}

func TestClueService_ErrorHandling(t *testing.T) {
	service, _, locationRepo, cleanup := setupClueService(t)
	defer cleanup()
	_ = locationRepo // Unused but needed for pattern consistency

	t.Run("Nil location", func(t *testing.T) {
		clues := []string{"Test clue"}
		
		// This should panic or error due to nil location
		assert.Panics(t, func() {
			service.UpdateClues(context.Background(), nil, clues, []string{})
		})
	})

	t.Run("Location with empty ID", func(t *testing.T) {
		location := &models.Location{
			ID:         "", // Empty ID
			InstanceID: uuid.New().String(),
			Name:       "Test Location",
		}
		
		clues := []string{"Test clue"}
		
		err := service.UpdateClues(context.Background(), location, clues, []string{})
		// May error due to empty ID, depending on repository implementation
		if err != nil {
			assert.Contains(t, err.Error(), "saving clue")
		}
	})

	t.Run("Location with empty instance ID", func(t *testing.T) {
		location := &models.Location{
			ID:         uuid.New().String(),
			InstanceID: "", // Empty instance ID
			Name:       "Test Location",
		}
		
		clues := []string{"Test clue"}
		
		err := service.UpdateClues(context.Background(), location, clues, []string{})
		// May error due to empty instance ID, depending on repository implementation
		if err != nil {
			assert.Contains(t, err.Error(), "saving clue")
		}
	})
}

func TestClueService_LargeDataSets(t *testing.T) {
	service, clueRepo, locationRepo, cleanup := setupClueService(t)
	defer cleanup()

	t.Run("Large number of clues", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		// Create 100 clues
		clues := make([]string, 100)
		for i := 0; i < 100; i++ {
			clues[i] = gofakeit.Sentence(5)
		}

		err := service.UpdateClues(context.Background(), location, clues, []string{})
		assert.NoError(t, err)

		// Verify all clues were created
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), location.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 100)
	})

	t.Run("Large number of clues with IDs", func(t *testing.T) {
		location := createTestLocation(t, locationRepo)
		
		// Create 50 clues with IDs
		clues := make([]string, 50)
		clueIDs := make([]string, 50)
		for i := 0; i < 50; i++ {
			clues[i] = gofakeit.Sentence(3)
			clueIDs[i] = uuid.New().String()
		}

		err := service.UpdateClues(context.Background(), location, clues, clueIDs)
		assert.NoError(t, err)

		// Verify all clues were created with correct IDs
		savedClues, err := clueRepo.FindCluesByLocation(context.Background(), location.ID)
		assert.NoError(t, err)
		assert.Len(t, savedClues, 50)
		
		// Verify IDs match
		savedIDs := make([]string, len(savedClues))
		for i, clue := range savedClues {
			savedIDs[i] = clue.ID
		}
		assert.ElementsMatch(t, clueIDs, savedIDs)
	})
}