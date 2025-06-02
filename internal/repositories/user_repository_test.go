package repositories_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v3/internal/repositories"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/stretchr/testify/assert"
)

func TestUserRepository_Create(t *testing.T) {
	cleanup := models.SetupTestDB(t)
	defer cleanup()

	repo := repositories.NewUserRepository()
	ctx := context.Background()

	user := &models.User{
		Name:     "John Doe",
		Email:    "john.doe@example.com",
		Password: "hashed_password",
		Provider: "local",
	}

	err := repo.Create(ctx, user)
	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)
}

func TestUserRepository_GetUserByEmail(t *testing.T) {
	cleanup := models.SetupTestDB(t)
	defer cleanup()

	repo := repositories.NewUserRepository()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     "John Doe",
		Email:    "john.doe@example.com",
		Password: "hashed_password",
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Test GetUserByEmail
	fetchedUser, err := repo.FindByEmail(ctx, "john.doe@example.com")
	assert.NoError(t, err)
	assert.Equal(t, user.Email, fetchedUser.Email)
	assert.Equal(t, user.Name, fetchedUser.Name)
}

func TestUserRepository_Update(t *testing.T) {
	cleanup := models.SetupTestDB(t)
	defer cleanup()

	repo := repositories.NewUserRepository()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     "John Doe",
		Email:    "john.doe@example.com",
		Password: "hashed_password",
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Update user
	user.Name = "John Updated"
	err = repo.Update(ctx, user)
	assert.NoError(t, err)

	// Verify update
	fetchedUser, err := repo.FindByEmail(ctx, "john.doe@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", fetchedUser.Name)
}

func TestUserRepository_FindUserByID(t *testing.T) {
	cleanup := models.SetupTestDB(t)
	defer cleanup()

	repo := repositories.NewUserRepository()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     "John Doe",
		Email:    "john.doe@example.com",
		Password: "hashed_password",
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Test FindUserByID
	fetchedUser, err := repo.FindByID(ctx, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, fetchedUser.ID)
	assert.Equal(t, user.Email, fetchedUser.Email)
}

func TestUserRepository_GetUserByEmailAndProvider(t *testing.T) {
	cleanup := models.SetupTestDB(t)
	defer cleanup()

	repo := repositories.NewUserRepository()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     "John Doe",
		Email:    "john.doe@example.com",
		Password: "hashed_password",
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Test GetUserByEmailAndProvider
	fetchedUser, err := repo.FindByEmailAndProvider(ctx, "john.doe@example.com", "local")
	assert.NoError(t, err)
	assert.Equal(t, user.Email, fetchedUser.Email)
	assert.Equal(t, user.Provider, fetchedUser.Provider)
}
