package services_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupInstanceService(t *testing.T) (services.InstanceService, services.UserService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	// Initialize repositories
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	clueRepo := repositories.NewClueRepository(dbc)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	markerService := services.NewMarkerService(markerRepo)

	// Initialize services
	locationService := services.NewLocationService(clueRepo, locationRepo, markerRepo, blockRepo, markerService)
	teamService := services.NewTeamService(teamRepo, checkInRepo, blockStateRepo, locationRepo)
	userService := services.NewUserService(userRepo, instanceRepo)
	instanceService := services.NewInstanceService(
		locationService, *teamService, instanceRepo, instanceSettingsRepo,
	)

	return instanceService, *userService, cleanup
}

func TestInstanceService(t *testing.T) {
	svc, userService, cleanup := setupInstanceService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	err := userService.CreateUser(context.Background(), user, "password")
	assert.NoError(t, err)
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
				instance, err := svc.CreateInstance(context.Background(), tc.instanceName, tc.user)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, instance)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, instance)
					assert.Equal(t, tc.instanceName, instance.Name)
				}
			})
		}
	})

	t.Run("DuplicateInstance", func(t *testing.T) {
		instance, _ := svc.CreateInstance(context.Background(), "Game1", user)

		tests := []struct {
			name       string
			instanceID string
			newName    string
			user       *models.User
			wantErr    bool
		}{
			{"Valid Duplicate", instance.ID, "Game1Copy", user, false},
			{"Empty Name", instance.ID, "", user, true},
			{"Invalid ID", "invalid-id", "Game2", user, true},
			{"Nil User", instance.ID, "Game3", nil, true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				duplicatedInstance, err := svc.DuplicateInstance(context.Background(), tc.user, tc.instanceID, tc.newName)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, duplicatedInstance)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, duplicatedInstance)
					assert.Equal(t, tc.newName, duplicatedInstance.Name)
				}
			})
		}
	})

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
				ids, err := svc.FindInstanceIDsForUser(context.Background(), tc.userID)
				if tc.wantErr {
					assert.Error(t, err)
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
				instances, err := svc.FindByUserID(context.Background(), tc.userID)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, instances)
				}
			})
		}
	})
}
