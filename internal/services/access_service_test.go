package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type AccessService interface {
	CanAdminAccessBlock(ctx context.Context, userID, blockID string) (bool, error)
	CanAdminAccessQuest(ctx context.Context, userID, questID string) (bool, error)
	CanAdminAccessObjective(ctx context.Context, userID, objectiveID string) (bool, error)
}

func setupAccessService(t *testing.T) (AccessService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewQuestRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	accessService := services.NewAccessService(blockRepo, instanceRepo, objectiveRepo)

	return accessService, cleanup
}

func TestAccessService_CanAdminAccessQuest(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Valid user and instance access", func(t *testing.T) {
		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		// This will likely return false due to no data setup, but validates the logic
		canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, questID)

		// Should not error with valid inputs
		require.NoError(t, err)
		assert.False(t, canAccess) // Expected false since no instances exist for user
	})

	t.Run("Empty user ID", func(t *testing.T) {
		questID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessQuest(context.Background(), "", questID)

		require.Error(t, err)
		assert.Equal(t, services.ErrUserNotAuthenticated, err)
		assert.False(t, canAccess)
	})

	t.Run("Empty instance ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Both empty user and instance ID", func(t *testing.T) {
		canAccess, err := service.CanAdminAccessQuest(context.Background(), "", "")

		require.Error(t, err)
		assert.Equal(t, services.ErrUserNotAuthenticated, err)
		assert.False(t, canAccess)
	})

	t.Run("User with multiple instances", func(t *testing.T) {
		userID := gofakeit.UUID()
		instanceID1 := gofakeit.UUID()
		instanceID2 := gofakeit.UUID()

		// Test with different instance IDs
		canAccess1, err1 := service.CanAdminAccessQuest(context.Background(), userID, instanceID1)
		canAccess2, err2 := service.CanAdminAccessQuest(context.Background(), userID, instanceID2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.False(t, canAccess1)
		assert.False(t, canAccess2)
	})

	t.Run("Whitespace in user ID", func(t *testing.T) {
		questID := gofakeit.UUID()

		// Test with whitespace user ID
		canAccess, err := service.CanAdminAccessQuest(context.Background(), "   ", questID)

		// Should pass validation (non-empty string)
		require.NoError(t, err)
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in instance ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		// Test with whitespace instance ID
		canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, "   ")

		// Should pass validation (non-empty string)
		require.NoError(t, err)
		assert.False(t, canAccess)
	})
}

func TestAccessService_CanAdminAccessBlock(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Valid user and block access", func(t *testing.T) {
		userID := gofakeit.UUID()
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, blockID)

		// May error due to block not existing, but shouldn't be validation error
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		} else {
			assert.False(t, canAccess)
		}
	})

	t.Run("Empty user ID", func(t *testing.T) {
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), "", blockID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Empty block ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "block ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Both empty user and block ID", func(t *testing.T) {
		canAccess, err := service.CanAdminAccessBlock(context.Background(), "", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
		assert.False(t, canAccess)
	})

	t.Run("Non-existent block", func(t *testing.T) {
		userID := gofakeit.UUID()
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, blockID)

		// Should error due to block not existing in database
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in user ID", func(t *testing.T) {
		blockID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), "   ", blockID)

		// Should pass validation (non-empty string)
		if err != nil {
			require.NotContains(t, err.Error(), "user ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Whitespace in block ID", func(t *testing.T) {
		userID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessBlock(context.Background(), userID, "   ")

		// Should pass validation (non-empty string)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_ValidationEdgeCases(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Very long IDs", func(t *testing.T) {
		longID := ""
		var longIDSb373 strings.Builder
		for range 1000 {
			longIDSb373.WriteString("a")
		}
		longID += longIDSb373.String()
		userID := gofakeit.UUID()

		// Test with very long instance ID
		canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, longID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		// Test with very long block ID
		canAccess, err = service.CanAdminAccessBlock(context.Background(), userID, longID)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Special characters in IDs", func(t *testing.T) {
		userID := gofakeit.UUID()
		specialID := "!@#$%^&*()_+-=[]{}|;':\",./<>?"

		// Test with special characters
		canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, specialID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessBlock(context.Background(), userID, specialID)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})

	t.Run("Unicode characters in IDs", func(t *testing.T) {
		userID := gofakeit.UUID()
		unicodeID := "测试🚀emoji"

		// Test with unicode characters
		canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, unicodeID)
		require.NoError(t, err)
		assert.False(t, canAccess)

		canAccess, err = service.CanAdminAccessBlock(context.Background(), userID, unicodeID)
		if err != nil {
			require.NotContains(t, err.Error(), "block ID cannot be empty")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_ContextCancellation(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		canAccess, err := service.CanAdminAccessQuest(ctx, userID, questID)

		// Should handle cancelled context gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		}
		assert.False(t, canAccess)
	})
}

func TestAccessService_ConcurrentAccess(t *testing.T) {
	service, cleanup := setupAccessService(t)
	defer cleanup()

	t.Run("Concurrent access checks", func(t *testing.T) {
		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		// Run multiple concurrent access checks
		results := make(chan bool, 10)
		errors := make(chan error, 10)

		for range 10 {
			go func() {
				canAccess, err := service.CanAdminAccessQuest(context.Background(), userID, questID)
				results <- canAccess
				errors <- err
			}()
		}

		// Collect results
		for range 10 {
			canAccess := <-results
			err := <-errors
			require.NoError(t, err)
			assert.False(t, canAccess)
		}
	})
}

func TestAccessService_CanAdminAccessBlockOwner(t *testing.T) {
	// This test requires full integration setup with database
	dbc, cleanup := setupDB(t)
	defer cleanup()

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewQuestRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	service := services.NewAccessService(blockRepo, instanceRepo, objectiveRepo)

	ctx := context.Background()

	t.Run("Owner can access start block through instance", func(t *testing.T) {
		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		// Create parent user for FK constraint
		insertTestUser(t, dbc, userID)

		// Create instance owned by user using repository
		inst := &models.Quest{
			ID:     questID,
			Name:   gofakeit.Word(),
			UserID: userID,
		}
		err := instanceRepo.Create(ctx, inst)
		require.NoError(t, err)

		// Test access to start block owner (instance)
		canAccess, err := service.CanAdminAccessBlockOwner(ctx, userID, questID, blocks.ContextStart)
		require.NoError(t, err)
		assert.True(t, canAccess, "User should have access to their own instance for start blocks")
	})

	t.Run("Owner can access finish block through instance", func(t *testing.T) {
		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		// Create parent user for FK constraint
		insertTestUser(t, dbc, userID)

		// Create instance owned by user using repository
		inst := &models.Quest{
			ID:     questID,
			Name:   gofakeit.Word(),
			UserID: userID,
		}
		err := instanceRepo.Create(ctx, inst)
		require.NoError(t, err)

		// Test access to finish block owner (instance)
		canAccess, err := service.CanAdminAccessBlockOwner(ctx, userID, questID, blocks.ContextFinish)
		require.NoError(t, err)
		assert.True(t, canAccess, "User should have access to their own instance for finish blocks")
	})

	t.Run("Owner can access objective proof/reveal block through objective", func(t *testing.T) {
		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		insertTestUser(t, dbc, userID)

		inst := &models.Quest{
			ID:     questID,
			Name:   gofakeit.Word(),
			UserID: userID,
		}
		err := instanceRepo.Create(ctx, inst)
		require.NoError(t, err)

		obj := &models.Objective{
			ID:      gofakeit.UUID(),
			QuestID: questID,
			Slug:    gofakeit.LetterN(8),
			Title:   gofakeit.Word(),
		}
		_, err = dbc.NewInsert().Model(obj).Exec(ctx)
		require.NoError(t, err)

		canAccess, err := service.CanAdminAccessBlockOwner(ctx, userID, obj.ID, blocks.ContextObjectiveProof)
		require.NoError(t, err)
		assert.True(t, canAccess, "User should have access to objective proof blocks in their own instance")

		canAccess, err = service.CanAdminAccessBlockOwner(ctx, userID, obj.ID, blocks.ContextObjectiveReveal)
		require.NoError(t, err)
		assert.True(t, canAccess, "User should have access to objective reveal blocks in their own instance")
	})

	t.Run("Non-owner cannot access objective", func(t *testing.T) {
		userID := gofakeit.UUID()
		otherUserID := gofakeit.UUID()
		questID := gofakeit.UUID()

		insertTestUser(t, dbc, userID)
		insertTestUser(t, dbc, otherUserID)

		inst := &models.Quest{
			ID:     questID,
			Name:   gofakeit.Word(),
			UserID: otherUserID,
		}
		err := instanceRepo.Create(ctx, inst)
		require.NoError(t, err)

		obj := &models.Objective{
			ID:      gofakeit.UUID(),
			QuestID: questID,
			Slug:    gofakeit.LetterN(8),
			Title:   gofakeit.Word(),
		}
		_, err = dbc.NewInsert().Model(obj).Exec(ctx)
		require.NoError(t, err)

		canAccess, err := service.CanAdminAccessBlockOwner(ctx, userID, obj.ID, blocks.ContextObjectiveProof)
		require.NoError(t, err)
		assert.False(t, canAccess, "User should not have access to another user's objective")
	})

	t.Run("Non-owner cannot access instance", func(t *testing.T) {
		userID := gofakeit.UUID()
		otherUserID := gofakeit.UUID()
		questID := gofakeit.UUID()

		// Create parent users for FK constraints
		insertTestUser(t, dbc, userID)
		insertTestUser(t, dbc, otherUserID)

		// Create instance owned by another user using repository
		inst := &models.Quest{
			ID:     questID,
			Name:   gofakeit.Word(),
			UserID: otherUserID,
		}
		err := instanceRepo.Create(ctx, inst)
		require.NoError(t, err)

		// Test access to start block - should fail
		canAccess, err := service.CanAdminAccessBlockOwner(ctx, userID, questID, blocks.ContextStart)
		require.NoError(t, err)
		assert.False(t, canAccess, "User should not have access to another user's instance")
	})

	t.Run("Context routing: start and finish -> instance, objective contexts -> objective", func(t *testing.T) {
		userID := gofakeit.UUID()
		questID := gofakeit.UUID()

		// Create parent rows for FK constraints
		insertTestUser(t, dbc, userID)

		// Create instance using repository
		inst := &models.Quest{
			ID:     questID,
			Name:   gofakeit.Word(),
			UserID: userID,
		}
		err := instanceRepo.Create(ctx, inst)
		require.NoError(t, err)

		obj := &models.Objective{
			ID:      gofakeit.UUID(),
			QuestID: questID,
			Slug:    gofakeit.LetterN(8),
			Title:   gofakeit.Word(),
		}
		_, err = dbc.NewInsert().Model(obj).Exec(ctx)
		require.NoError(t, err)

		// Test start context routes to instance check
		canAccess, err := service.CanAdminAccessBlockOwner(ctx, userID, questID, blocks.ContextStart)
		require.NoError(t, err)
		assert.True(t, canAccess)

		// Test finish context routes to instance check
		canAccess, err = service.CanAdminAccessBlockOwner(ctx, userID, questID, blocks.ContextFinish)
		require.NoError(t, err)
		assert.True(t, canAccess)

		// Test objective proof context routes to objective check.
		canAccess, err = service.CanAdminAccessBlockOwner(ctx, userID, obj.ID, blocks.ContextObjectiveProof)
		require.NoError(t, err)
		assert.True(t, canAccess)

		// Test objective reveal context routes to objective check.
		canAccess, err = service.CanAdminAccessBlockOwner(ctx, userID, obj.ID, blocks.ContextObjectiveReveal)
		require.NoError(t, err)
		assert.True(t, canAccess)
	})
}
