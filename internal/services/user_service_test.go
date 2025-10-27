package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/internal/services"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserService(t *testing.T) (services.UserService, repositories.InstanceRepository, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	instanceRepo := repositories.NewInstanceRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	userService := services.NewUserService(userRepo, instanceRepo)
	return *userService, instanceRepo, cleanup
}

func TestCreateUser(t *testing.T) {
	service, _, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, false, 12)

	user := &models.User{
		Email:    email,
		Password: password,
	}
	err := service.CreateUser(context.Background(), user, password)

	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.NotEqual(t, password, user.Password) // Ensure password is transformed/hashed
}

func TestCreateUser_PasswordsDoNotMatch(t *testing.T) {
	service, _, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, false, 12)
	user := &models.User{
		Email:    email,
		Password: password,
	}
	err := service.CreateUser(context.Background(), user, "differentPassword")

	require.Error(t, err)
	assert.Equal(t, services.ErrPasswordsDoNotMatch, err)
}

func TestGetUserByEmail(t *testing.T) {
	service, _, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, false, 12)

	user := &models.User{
		Email:    email,
		Password: password,
	}
	err := service.CreateUser(context.Background(), user, password)
	require.NoError(t, err)

	retrievedUser, err := service.GetUserByEmail(context.Background(), email)

	require.NoError(t, err)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.ID, retrievedUser.ID)
}

func TestUpdateUser(t *testing.T) {
	service, _, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, false, 12)

	user := &models.User{
		Email:    email,
		Password: password,
	}
	err := service.CreateUser(context.Background(), user, password)
	require.NoError(t, err)

	newName := gofakeit.Name()
	user.Name = newName
	err = service.UpdateUser(context.Background(), user)
	require.NoError(t, err)

	retrievedUser, err := service.GetUserByEmail(context.Background(), email)
	assert.NotEmpty(t, retrievedUser.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, retrievedUser.Name)
}

func TestUpdateUserProfile(t *testing.T) {
	service, _, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, false, 12)

	// Create initial user with some data
	user := &models.User{
		Email:    email,
		Password: password,
		Name:     "Initial Name",
	}
	user.DisplayName.String = "Initial Display"
	user.DisplayName.Valid = true
	user.WorkType.String = "formal_education"
	user.WorkType.Valid = true

	err := service.CreateUser(context.Background(), user, password)
	require.NoError(t, err)

	// Test cases
	testCases := []struct {
		name     string
		profile  map[string]string
		validate func(t *testing.T, user *models.User)
	}{
		{
			name: "Full profile update",
			profile: map[string]string{
				"name":         "John Doe",
				"display_name": "JD",
				"show_email":   "on",
				"work_type":    "corporate_training",
			},
			validate: func(t *testing.T, user *models.User) {
				assert.Equal(t, "John Doe", user.Name)
				assert.True(t, user.DisplayName.Valid)
				assert.Equal(t, "JD", user.DisplayName.String)
				assert.True(t, user.ShareEmail)
				assert.True(t, user.WorkType.Valid)
				assert.Equal(t, "corporate_training", user.WorkType.String)
			},
		},
		{
			name: "Only name and email setting update - preserves other fields",
			profile: map[string]string{
				"name":       "Jane Smith",
				"show_email": "", // Empty means unchecked
			},
			validate: func(t *testing.T, user *models.User) {
				// Name and ShareEmail should change
				assert.Equal(t, "Jane Smith", user.Name)
				assert.False(t, user.ShareEmail)

				// Other fields should be preserved
				assert.True(t, user.DisplayName.Valid)
				assert.Equal(t, "JD", user.DisplayName.String)
				assert.True(t, user.WorkType.Valid)
				assert.Equal(t, "corporate_training", user.WorkType.String)
			},
		},
		{
			name: "Only work type update - preserves other fields",
			profile: map[string]string{
				"work_type":       "other",
				"other_work_type": "Museum Curator",
			},
			validate: func(t *testing.T, user *models.User) {
				// Work type should change
				assert.True(t, user.WorkType.Valid)
				assert.Equal(t, "Museum Curator", user.WorkType.String)

				// Other fields should be preserved
				assert.Equal(t, "Jane Smith", user.Name)
				assert.True(t, user.DisplayName.Valid)
				assert.Equal(t, "JD", user.DisplayName.String)
				assert.False(t, user.ShareEmail)
			},
		},
		{
			name: "Empty display name - only affects that field",
			profile: map[string]string{
				"display_name": "",
			},
			validate: func(t *testing.T, user *models.User) {
				// Display name should be cleared
				assert.False(t, user.DisplayName.Valid)

				// Other fields should be preserved
				assert.Equal(t, "Jane Smith", user.Name)
				assert.False(t, user.ShareEmail)
				assert.True(t, user.WorkType.Valid)
				assert.Equal(t, "Museum Curator", user.WorkType.String)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get a fresh copy of the user for each test
			currentUser, err := service.GetUserByEmail(context.Background(), email)
			require.NoError(t, err)

			// Update the profile
			updateErr := service.UpdateUserProfile(context.Background(), currentUser, tc.profile)
			require.NoError(t, updateErr)

			// Retrieve the updated user
			updatedUser, getErr := service.GetUserByEmail(context.Background(), email)
			require.NoError(t, getErr)

			// Validate fields
			tc.validate(t, updatedUser)
		})
	}
}

func TestChangePassword(t *testing.T) {
	service, _, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	oldPassword := gofakeit.Password(true, true, true, true, false, 12)
	newPassword := gofakeit.Password(true, true, true, true, false, 12)

	// Create a user with email provider
	user := &models.User{
		Email:    email,
		Password: oldPassword,
		Provider: models.ProviderEmail,
	}
	err := service.CreateUser(context.Background(), user, oldPassword)
	require.NoError(t, err)

	// Retrieve user to get the hashed password
	retrievedUser, err := service.GetUserByEmail(context.Background(), email)
	require.NoError(t, err)
	oldHashedPassword := retrievedUser.Password

	// Test cases
	t.Run("Successful password change", func(t *testing.T) {
		err = service.ChangePassword(context.Background(), retrievedUser, oldPassword, newPassword, newPassword)
		require.NoError(t, err)

		// Verify the password was changed in the database
		updatedUser, getErr := service.GetUserByEmail(context.Background(), email)
		require.NoError(t, getErr)
		assert.NotEqual(t, oldHashedPassword, updatedUser.Password)
	})

	t.Run("Incorrect old password", func(t *testing.T) {
		err = service.ChangePassword(context.Background(), retrievedUser, "wrongPassword", newPassword, newPassword)
		require.Error(t, err)
		assert.Equal(t, services.ErrIncorrectOldPassword, err)
	})

	t.Run("Passwords don't match", func(t *testing.T) {
		err = service.ChangePassword(context.Background(), retrievedUser, oldPassword, newPassword, "differentPassword")
		require.Error(t, err)
		assert.Equal(t, services.ErrPasswordsDoNotMatch, err)
	})

	t.Run("Empty new password", func(t *testing.T) {
		err = service.ChangePassword(context.Background(), retrievedUser, oldPassword, "", "")
		require.Error(t, err)
		assert.Equal(t, services.ErrEmptyPassword, err)
	})

	// Create a user with Google provider
	googleUser := &models.User{
		Email:    gofakeit.Email(),
		Password: "not-used-for-google",
		Provider: models.ProviderGoogle,
	}
	err = service.CreateUser(context.Background(), googleUser, "not-used-for-google")
	require.NoError(t, err)

	t.Run("Google user cannot change password", func(t *testing.T) {
		err = service.ChangePassword(context.Background(), googleUser, "any-password", newPassword, newPassword)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot change password for SSO accounts")
	})
}

func TestUserService_SwitchInstance(t *testing.T) {
	service, instanceRepo, cleanup := setupUserService(t)
	defer cleanup()

	email := gofakeit.Email()
	user := &models.User{Email: email, Password: "password", CurrentInstanceID: "instance123"}
	err := service.CreateUser(context.Background(), user, "password")
	require.NoError(t, err)

	err = instanceRepo.Create(context.Background(), &models.Instance{ID: "instance789", Name: "Game1", UserID: user.ID})
	require.NoError(t, err)

	t.Run("SwitchInstance", func(t *testing.T) {
		tests := []struct {
			name       string
			instanceID string
			user       *models.User
			wantErr    bool
		}{
			{"Valid Instance", "instance789", user, false},
			{"Empty ID", "", user, true},
			{"Nil User", "instance789", nil, true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				switchErr := service.SwitchInstance(context.Background(), tc.user, tc.instanceID)
				if tc.wantErr {
					require.Error(t, switchErr)
				} else {
					require.NoError(t, switchErr)
					assert.Equal(t, tc.instanceID, tc.user.CurrentInstanceID)
				}
			})
		}
	})
}
