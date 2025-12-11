package services_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupInstanceService(t *testing.T) (services.InstanceService, services.UserService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	// Initialize repositories
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	// Initialize services
	userService := services.NewUserService(userRepo, instanceRepo)
	instanceService := services.NewInstanceService(
		instanceRepo, instanceSettingsRepo, blockRepo,
	)

	return *instanceService, *userService, cleanup
}

func setupInstanceServiceWithBlockRepo(
	t *testing.T,
) (
	services.InstanceService,
	services.UserService,
	repositories.BlockRepository,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	// Initialize repositories
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	// Initialize services
	userService := services.NewUserService(userRepo, instanceRepo)
	instanceService := services.NewInstanceService(
		instanceRepo, instanceSettingsRepo, blockRepo,
	)

	return *instanceService, *userService, blockRepo, cleanup
}

func TestInstanceService_CreateInstance_DefaultBlocks(t *testing.T) {
	svc, userService, blockRepo, cleanup := setupInstanceServiceWithBlockRepo(t)
	defer cleanup()

	user := &models.User{Email: "blockstest@example.com", Password: "password"}
	err := userService.CreateUser(context.Background(), user, "password")
	require.NoError(t, err)

	instance, err := svc.CreateInstance(context.Background(), "Test Game", user)
	require.NoError(t, err)
	require.NotNil(t, instance)

	t.Run("creates default lobby blocks", func(t *testing.T) {
		lobbyBlocks, lobbyErr := blockRepo.FindByOwnerIDAndContext(
			context.Background(),
			instance.ID,
			blocks.ContextLobby,
		)
		require.NoError(t, lobbyErr)
		assert.Len(t, lobbyBlocks, 7, "should create 7 lobby blocks")

		// Verify block types in order
		expectedTypes := []string{
			"header",
			"game_status_alert",
			"divider",
			"markdown",
			"divider",
			"team_name",
			"start_game_button",
		}
		for i, block := range lobbyBlocks {
			assert.Equal(t, expectedTypes[i], block.GetType(), "block %d should be %s", i, expectedTypes[i])
			assert.Equal(t, i, block.GetOrder(), "block %d should have order %d", i, i)
		}

		// Verify header block content
		headerBlock, ok := lobbyBlocks[0].(*blocks.HeaderBlock)
		require.True(t, ok, "first block should be HeaderBlock")
		assert.Equal(t, "Test Game", headerBlock.TitleText)
		assert.Equal(t, "map-pin-check-inside", headerBlock.Icon)

		// Verify game status alert block content
		gameStatusBlock, ok := lobbyBlocks[1].(*blocks.GameStatusAlertBlock)
		require.True(t, ok, "second block should be GameStatusAlertBlock")
		assert.Equal(t, "This game is not yet open.", gameStatusBlock.ClosedMessage)
		assert.True(t, gameStatusBlock.ShowCountdown)
	})

	t.Run("creates default finish blocks", func(t *testing.T) {
		finishBlocks, finishErr := blockRepo.FindByOwnerIDAndContext(
			context.Background(),
			instance.ID,
			blocks.ContextFinish,
		)
		require.NoError(t, finishErr)
		assert.Len(t, finishBlocks, 2, "should create 2 finish blocks")

		// Verify block types in order
		expectedTypes := []string{"header", "markdown"}
		for i, block := range finishBlocks {
			assert.Equal(t, expectedTypes[i], block.GetType(), "block %d should be %s", i, expectedTypes[i])
			assert.Equal(t, i, block.GetOrder(), "block %d should have order %d", i, i)
		}

		// Verify header block content
		headerBlock, ok := finishBlocks[0].(*blocks.HeaderBlock)
		require.True(t, ok, "first block should be HeaderBlock")
		assert.Equal(t, "Congratulations!", headerBlock.TitleText)
		assert.Equal(t, "party-popper", headerBlock.Icon)
	})
}

func TestInstanceService(t *testing.T) { //nolint:gocognit // Test complexity is acceptable
	svc, userService, cleanup := setupInstanceService(t)
	defer cleanup()

	user := &models.User{Email: "instancetest@example.com", Password: "password", CurrentInstanceID: "instance123"}
	createErr := userService.CreateUser(context.Background(), user, "password")
	require.NoError(t, createErr)
	assert.NotEmpty(t, user.ID)

	t.Run("CreateInstance", func(t *testing.T) {
		tests := []struct {
			name         string
			instanceName string
			user         *models.User
			wantErr      bool
		}{
			{"Valid Instance", "Game1", user, false},
			{"Empty Name", "", user, true},
			{"Nil User", "Game2", nil, true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				instance, instanceErr := svc.CreateInstance(context.Background(), tc.instanceName, tc.user)
				if tc.wantErr {
					require.Error(t, instanceErr)
					assert.Nil(t, instance)
				} else {
					require.NoError(t, instanceErr)
					assert.NotNil(t, instance)
					assert.Equal(t, tc.instanceName, instance.Name)
				}
			})
		}
	})

	// DuplicateInstance tests moved to duplication_service_test.go

	t.Run("FindInstanceIDsForUser", func(t *testing.T) {
		_, _ = svc.CreateInstance(context.Background(), "GameA", user)
		_, _ = svc.CreateInstance(context.Background(), "GameB", user)

		tests := []struct {
			name    string
			userID  string
			wantErr bool
		}{
			{"Valid User", user.ID, false},
			{"Invalid User", "non-existent", false}, // This is not an error, just an empty list
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ids, findErr := svc.FindInstanceIDsForUser(context.Background(), tc.userID)
				if tc.wantErr {
					require.Error(t, findErr)
					assert.Nil(t, ids)
				}
			})
		}
	})

	t.Run("FindByUserID", func(t *testing.T) {
		_, _ = svc.CreateInstance(context.Background(), "Game1", user)
		_, _ = svc.CreateInstance(context.Background(), "Game2", user)

		tests := []struct {
			name    string
			userID  string
			wantErr bool
		}{
			{"Valid User", user.ID, false},
			{"Invalid User", "non-existent", false}, // This is not an error, just an empty list
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				instances, findErr := svc.FindByUserID(context.Background(), tc.userID)
				if tc.wantErr {
					require.Error(t, findErr)
					assert.Nil(t, instances)
				}
			})
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		instance, err := svc.CreateInstance(context.Background(), "TestGetByID", user)
		require.NoError(t, err)

		tests := []struct {
			name       string
			instanceID string
			wantErr    bool
		}{
			{"Valid Instance ID", instance.ID, false},
			{"Invalid Instance ID", "non-existent", true},
			{"Empty Instance ID", "", true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				retrieved, getErr := svc.GetByID(context.Background(), tc.instanceID)
				if tc.wantErr {
					require.Error(t, getErr)
					assert.Nil(t, retrieved)
				} else {
					require.NoError(t, getErr)
					assert.NotNil(t, retrieved)
					assert.Equal(t, instance.ID, retrieved.ID)
					assert.Equal(t, instance.Name, retrieved.Name)
				}
			})
		}
	})

	t.Run("Update", func(t *testing.T) {
		instance, err := svc.CreateInstance(context.Background(), "OriginalName", user)
		require.NoError(t, err)

		t.Run("Valid Update", func(t *testing.T) {
			// Get the full instance first
			fullInstance, getErr := svc.GetByID(context.Background(), instance.ID)
			require.NoError(t, getErr)

			// Update the name
			fullInstance.Name = "UpdatedName"
			updateErr := svc.Update(context.Background(), fullInstance)
			require.NoError(t, updateErr)

			// Verify the update persisted
			updated, getErr := svc.GetByID(context.Background(), fullInstance.ID)
			require.NoError(t, getErr)
			assert.Equal(t, "UpdatedName", updated.Name)
		})

		t.Run("Nil Instance", func(t *testing.T) {
			updateErr := svc.Update(context.Background(), nil)
			require.Error(t, updateErr)
		})

		t.Run("Empty Name", func(t *testing.T) {
			fullInstance, getErr := svc.GetByID(context.Background(), instance.ID)
			require.NoError(t, getErr)

			fullInstance.Name = ""
			updateErr := svc.Update(context.Background(), fullInstance)
			require.Error(t, updateErr)
		})
	})
}
