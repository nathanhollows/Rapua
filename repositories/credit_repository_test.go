package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCreditRepo(t *testing.T) (*repositories.CreditRepository, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	creditRepo := repositories.NewCreditRepository(dbc)

	return creditRepo, dbc, cleanup
}

func createTestUser(t *testing.T, db *bun.DB, freeCredits, paidCredits int) models.User {
	t.Helper()

	user := models.User{
		ID:          gofakeit.UUID(),
		Name:        gofakeit.Name(),
		Email:       gofakeit.Email(),
		FreeCredits: freeCredits,
		PaidCredits: paidCredits,
		IsEducator:  false,
	}

	// Insert user directly into database for testing
	_, err := db.NewInsert().
		Model(&user).
		Exec(context.Background())
	require.NoError(t, err)

	return user
}

func TestCreditRepo_UpdateCredits(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user with initial credits
	user := createTestUser(t, db, 10, 5)

	// Update credits
	err := repo.UpdateCredits(ctx, user.ID, 8, 3)
	assert.NoError(t, err)

	// Verify credits were updated by querying the database directly
	var updatedUser models.User
	err = db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, 8, updatedUser.FreeCredits)
	assert.Equal(t, 3, updatedUser.PaidCredits)
}

func TestCreditRepo_UpdateCredits_NonExistentUser(t *testing.T) {
	repo, _, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Try to update credits for non-existent user
	err := repo.UpdateCredits(ctx, gofakeit.UUID(), 10, 5)

	// Should not return an error but should not affect any rows
	assert.NoError(t, err)
}

func TestCreditRepo_UpdateCredits_ZeroCredits(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user with initial credits
	user := createTestUser(t, db, 5, 3)

	// Update to zero credits
	err := repo.UpdateCredits(ctx, user.ID, 0, 0)
	assert.NoError(t, err)

	// Verify credits were zeroed
	var updatedUser models.User
	err = db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, updatedUser.FreeCredits)
	assert.Equal(t, 0, updatedUser.PaidCredits)
}

func TestCreditRepo_UpdateCredits_NegativeCredits(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user with initial credits
	user := createTestUser(t, db, 5, 3)

	// Try to set negative credits - this should fail due to database constraints
	err := repo.UpdateCredits(ctx, user.ID, -1, -2)
	assert.Error(t, err)

	// Verify credits were NOT changed due to constraint violation
	var updatedUser models.User
	err = db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	// Credits should remain unchanged
	assert.Equal(t, 5, updatedUser.FreeCredits)
	assert.Equal(t, 3, updatedUser.PaidCredits)
}

func TestCreditRepo_UpdateCredits_MultipleUsers(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple test users
	user1 := createTestUser(t, db, 10, 5)
	user2 := createTestUser(t, db, 20, 15)

	// Update credits for first user
	err := repo.UpdateCredits(ctx, user1.ID, 8, 3)
	assert.NoError(t, err)

	// Update credits for second user
	err = repo.UpdateCredits(ctx, user2.ID, 18, 13)
	assert.NoError(t, err)

	// Verify both users were updated correctly
	var updatedUser1, updatedUser2 models.User

	err = db.NewSelect().
		Model(&updatedUser1).
		Where("id = ?", user1.ID).
		Scan(ctx)
	require.NoError(t, err)

	err = db.NewSelect().
		Model(&updatedUser2).
		Where("id = ?", user2.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, 8, updatedUser1.FreeCredits)
	assert.Equal(t, 3, updatedUser1.PaidCredits)
	assert.Equal(t, 18, updatedUser2.FreeCredits)
	assert.Equal(t, 13, updatedUser2.PaidCredits)
}

func TestCreditRepo_UpdateCredits_ConcurrentUpdates(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	user := createTestUser(t, db, 10, 5)

	// Simulate concurrent updates
	done := make(chan bool, 2)

	go func() {
		err := repo.UpdateCredits(ctx, user.ID, 8, 3)
		assert.NoError(t, err)
		done <- true
	}()

	go func() {
		err := repo.UpdateCredits(ctx, user.ID, 6, 1)
		assert.NoError(t, err)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify that one of the updates succeeded (last writer wins)
	var updatedUser models.User
	err := db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	// Should be one of the two update values
	assert.True(t,
		(updatedUser.FreeCredits == 8 && updatedUser.PaidCredits == 3) ||
			(updatedUser.FreeCredits == 6 && updatedUser.PaidCredits == 1))
}
