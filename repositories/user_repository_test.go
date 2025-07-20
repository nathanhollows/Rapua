package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupUserRepo(t *testing.T) (repositories.UserRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	userRepository := repositories.NewUserRepository(dbc)
	return userRepository, transactor, cleanup
}

func TestUserRepository_Create(t *testing.T) {
	repo, _, cleanup := setupUserRepo(t)
	defer cleanup()
	ctx := context.Background()

	user := &models.User{
		Name:     gofakeit.Name(),
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 12),
		Provider: "local",
	}

	err := repo.Create(ctx, user)
	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)
}

func TestUserRepository_GetUserByEmail(t *testing.T) {
	repo, _, cleanup := setupUserRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     gofakeit.Name(),
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 12),
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Test GetUserByEmail
	fetchedUser, err := repo.GetByEmail(ctx, user.Email)
	assert.NoError(t, err)
	assert.Equal(t, user.Email, fetchedUser.Email)
	assert.Equal(t, user.Name, fetchedUser.Name)
}

func TestUserRepository_Update(t *testing.T) {
	repo, _, cleanup := setupUserRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     gofakeit.Name(),
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 12),
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Update user

	// // ID is immutable
	// // Provider is immutable
	// "name",
	// "email_token",
	// "email_token_expiry",
	// "email_verified",
	// "password",
	// "current_instance_id",
	// "updated_at").

	newName := gofakeit.Name()
	newEmailToken := gofakeit.UUID()
	newEmailTokenExpiry := sql.NullTime{
		Time:  time.Now().Add(time.Hour * 24).UTC(),
		Valid: true,
	}
	newEmailVerified := gofakeit.Bool()
	newPassword := gofakeit.Password(true, true, true, true, true, 12)
	newCurrentInstanceID := gofakeit.UUID()

	user.Name = newName
	user.EmailToken = newEmailToken
	user.EmailTokenExpiry = newEmailTokenExpiry
	user.EmailVerified = newEmailVerified
	user.Password = newPassword
	user.CurrentInstanceID = newCurrentInstanceID

	err = repo.Update(ctx, user)
	assert.NoError(t, err)

	// Verify update
	fetchedUser, err := repo.GetByEmail(ctx, user.Email)
	assert.NoError(t, err)
	assert.Equal(t, newName, fetchedUser.Name)
	assert.Equal(t, newEmailToken, fetchedUser.EmailToken)
	assert.NotEqual(t, user.EmailTokenExpiry, fetchedUser.EmailTokenExpiry)
	assert.Equal(t, newEmailVerified, fetchedUser.EmailVerified)
	assert.Equal(t, newPassword, fetchedUser.Password)
	assert.Equal(t, newCurrentInstanceID, fetchedUser.CurrentInstanceID)
}

func TestUserRepository_FindUserByID(t *testing.T) {
	repo, _, cleanup := setupUserRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     gofakeit.Name(),
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 12),
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Test FindUserByID
	fetchedUser, err := repo.GetByID(ctx, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, fetchedUser.ID)
	assert.Equal(t, user.Email, fetchedUser.Email)
}

func TestUserRepository_GetUserByEmailAndProvider(t *testing.T) {
	repo, _, cleanup := setupUserRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     gofakeit.Name(),
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 12),
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Test GetUserByEmailAndProvider
	fetchedUser, err := repo.GetByEmailAndProvider(ctx, user.Email, "local")
	assert.NoError(t, err)
	assert.Equal(t, user.Email, fetchedUser.Email)
	assert.Equal(t, user.Provider, fetchedUser.Provider)
}

func TestUserRepository_Delete(t *testing.T) {
	repo, transactor, cleanup := setupUserRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Seed user
	user := &models.User{
		Name:     gofakeit.Name(),
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 12),
		Provider: "local",
	}
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Delete user
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = repo.Delete(ctx, tx, user.ID)
	assert.NoError(t, err)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Verify user is deleted
	fetchedUser, err := repo.GetByEmail(ctx, user.Email)
	assert.Error(t, err)
	assert.Empty(t, fetchedUser)
}
