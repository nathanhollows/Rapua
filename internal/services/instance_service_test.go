package services_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
)

func setupInstanceService(t *testing.T) (services.InstanceService, services.UserService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

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

	// Initialize services
	locationService := services.NewLocationService(transactor, clueRepo, locationRepo, markerRepo, blockRepo)
	teamService := services.NewTeamService(transactor, teamRepo, checkInRepo, blockStateRepo, locationRepo)
	userService := services.NewUserService(transactor, userRepo, instanceRepo)
	instanceService := services.NewInstanceService(
		transactor,
		locationService, userService, teamService, instanceRepo, instanceSettingsRepo,
	)

	return instanceService, userService, cleanup
}

func TestInstanceService(t *testing.T) {
	svc, userService, cleanup := setupInstanceService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password"}
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

	t.Run("DeleteInstance", func(t *testing.T) {
		instance, _ := svc.CreateInstance(context.Background(), "GameToDelete", user)

		tests := []struct {
			name         string
			instanceID   string
			confirmName  string
			user         *models.User
			wantErr      bool
			expectedBool bool
		}{
			{"Invalid, currently in use", instance.ID, "GameToDelete", user, true, true},
			{"Mismatched Confirmation", instance.ID, "WrongName", user, true, false},
			{"Invalid Instance ID", "invalid-id", "GameToDelete", user, true, false},
			{"Nil User", instance.ID, "GameToDelete", nil, true, false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				success, err := svc.DeleteInstance(context.Background(), tc.user, tc.instanceID, tc.confirmName)
				if tc.wantErr {
					assert.Error(t, err)
					assert.False(t, success)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.expectedBool, success)
				}
			})
		}
	})

	t.Run("SwitchInstance", func(t *testing.T) {
		instance, _ := svc.CreateInstance(context.Background(), "GameToSwitch", user)

		tests := []struct {
			name       string
			instanceID string
			user       *models.User
			wantErr    bool
		}{
			{"Valid Switch", instance.ID, user, false},
			{"Invalid ID", "invalid-id", user, true},
			{"Nil User", instance.ID, nil, true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				switchedInstance, err := svc.SwitchInstance(context.Background(), tc.user, tc.instanceID)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, switchedInstance)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, switchedInstance)
					assert.Equal(t, instance.ID, switchedInstance.ID)
				}
			})
		}
	})
}
