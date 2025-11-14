package services_test

import (
	"context"
	"testing"

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

	// Initialize services
	userService := services.NewUserService(userRepo, instanceRepo)
	instanceService := services.NewInstanceService(
		instanceRepo, instanceSettingsRepo,
	)

	return *instanceService, *userService, cleanup
}

func TestInstanceService(t *testing.T) {
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
				instance, createErr := svc.CreateInstance(context.Background(), tc.instanceName, tc.user)
				if tc.wantErr {
					require.Error(t, createErr)
					assert.Nil(t, instance)
				} else {
					require.NoError(t, createErr)
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
