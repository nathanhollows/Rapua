package repositories_test

import (
	"context"
	"fmt"
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

func TestCreditRepo_AddCreditsWithTx(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user with initial credits
	user := createTestUser(t, db, 10, 5)

	// Add credits using a transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.AddCreditsWithTx(ctx, &tx, user.ID, 5, 3)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify credits were updated by querying the database directly
	var updatedUser models.User
	err = db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, 15, updatedUser.FreeCredits)
	assert.Equal(t, 8, updatedUser.PaidCredits)
}

func TestCreditRepo_AddCreditsWithTx_NonExistentUser(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Try to add credits for non-existent user
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.AddCreditsWithTx(ctx, &tx, gofakeit.UUID(), 10, 5)

	// Should return an error for non-existent user
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestCreditRepo_DeductOneCreditWithTx_FreeCreditsFirst(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user with both free and paid credits
	user := createTestUser(t, db, 5, 3)

	// Deduct one credit using a transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.DeductOneCreditWithTx(ctx, &tx, user.ID)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify free credits were deducted first
	var updatedUser models.User
	err = db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, 4, updatedUser.FreeCredits, "Free credits should be deducted first")
	assert.Equal(t, 3, updatedUser.PaidCredits, "Paid credits should remain unchanged")
}

func TestCreditRepo_DeductOneCreditWithTx_PaidCreditsWhenFreeIsZero(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user (will get default 10 free credits)
	user := createTestUser(t, db, 10, 0)

	// Manually set to 0 free credits and 3 paid credits
	_, err := db.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = ?", 0).
		Set("paid_credits = ?", 3).
		Where("id = ?", user.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Deduct one credit using a transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.DeductOneCreditWithTx(ctx, &tx, user.ID)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify paid credits were deducted
	var updatedUser models.User
	err = db.NewSelect().
		Model(&updatedUser).
		Where("id = ?", user.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, updatedUser.FreeCredits, "Free credits should remain zero")
	assert.Equal(t, 2, updatedUser.PaidCredits, "Paid credits should be deducted")
}

func TestCreditRepo_DeductOneCreditWithTx_InsufficientCredits(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user (will get default 10 free credits)
	user := createTestUser(t, db, 10, 0)

	// Manually set to 0 credits
	_, err := db.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = ?", 0).
		Set("paid_credits = ?", 0).
		Where("id = ?", user.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Try to deduct one credit using a transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.DeductOneCreditWithTx(ctx, &tx, user.ID)

	// Should return an error for insufficient credits
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient credits")
}

func TestCreditRepo_CreateCreditAdjustmentWithTx(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	user := createTestUser(t, db, 10, 5)

	// Create a credit adjustment using a transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	adjustment := &models.CreditAdjustments{
		ID:      gofakeit.UUID(),
		UserID:  user.ID,
		Credits: 5,
		Reason:  "Test adjustment",
	}

	err = repo.CreateCreditAdjustmentWithTx(ctx, &tx, adjustment)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify the adjustment was created
	adjustments, err := repo.GetCreditAdjustmentsByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.Equal(t, 5, adjustments[0].Credits)
	assert.Equal(t, "Test adjustment", adjustments[0].Reason)
}

func TestCreditRepo_GetCreditAdjustmentsByUserIDWithPagination(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	user := createTestUser(t, db, 10, 5)

	// Create multiple adjustments
	for i := 1; i <= 5; i++ {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		adjustment := &models.CreditAdjustments{
			ID:      gofakeit.UUID(),
			UserID:  user.ID,
			Credits: i,
			Reason:  fmt.Sprintf("Adjustment %d", i),
		}

		err = repo.CreateCreditAdjustmentWithTx(ctx, &tx, adjustment)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)
	}

	testCases := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
	}{
		{
			name:          "First page with limit 2",
			limit:         2,
			offset:        0,
			expectedCount: 2,
		},
		{
			name:          "Second page with limit 2",
			limit:         2,
			offset:        2,
			expectedCount: 2,
		},
		{
			name:          "Last page with remaining records",
			limit:         2,
			offset:        4,
			expectedCount: 1,
		},
		{
			name:          "Page beyond available records",
			limit:         10,
			offset:        10,
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adjustments, err := repo.GetCreditAdjustmentsByUserIDWithPagination(ctx, user.ID, tc.limit, tc.offset)
			require.NoError(t, err)
			assert.Len(t, adjustments, tc.expectedCount)
		})
	}
}

func TestCreditRepo_BulkUpdateCredits(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple users with 10 free credits each
	var userIDs []string
	for range 3 {
		user := createTestUser(t, db, 10, 0)
		userIDs = append(userIDs, user.ID)
	}

	// Create one user with different credits
	differentUser := createTestUser(t, db, 5, 0)

	// Bulk update users with 10 free credits to 15 free credits
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.BulkUpdateCredits(ctx, &tx, 10, 15, false)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify the bulk update affected the right users
	for _, userID := range userIDs {
		var user models.User
		err = db.NewSelect().
			Model(&user).
			Where("id = ?", userID).
			Scan(ctx)
		require.NoError(t, err)
		assert.Equal(t, 15, user.FreeCredits, "User should have updated credits")
	}

	// Verify the user with different credits was not affected
	var unchangedUser models.User
	err = db.NewSelect().
		Model(&unchangedUser).
		Where("id = ?", differentUser.ID).
		Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, unchangedUser.FreeCredits, "User with different credits should be unchanged")
}

func TestCreditRepo_BulkUpdateCreditUpdateNotices(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple users with 10 free credits each
	var userIDs []string
	for range 3 {
		user := createTestUser(t, db, 10, 0)
		userIDs = append(userIDs, user.ID)
	}

	// Bulk create credit adjustments for users with 10 credits
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.BulkUpdateCreditUpdateNotices(ctx, &tx, 10, 15, false, "Monthly top-up")
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify each user got a credit adjustment record
	for _, userID := range userIDs {
		adjustments, err := repo.GetCreditAdjustmentsByUserID(ctx, userID)
		require.NoError(t, err)
		require.Len(t, adjustments, 1)
		assert.Equal(t, 5, adjustments[0].Credits, "Adjustment should show difference (15-10=5)")
		assert.Equal(t, "Monthly top-up", adjustments[0].Reason)
	}
}

func TestCreditRepo_GetMostRecentCreditAdjustmentByReasonPrefix(t *testing.T) {
	repo, db, cleanup := setupCreditRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	user := createTestUser(t, db, 10, 5)

	// Create adjustments with different reasons at different times
	reasons := []string{
		"Monthly top-up 2025-01",
		"Monthly top-up 2025-02",
		"Purchase: 10 credits",
	}

	for _, reason := range reasons {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		adjustment := &models.CreditAdjustments{
			ID:      gofakeit.UUID(),
			UserID:  user.ID,
			Credits: 5,
			Reason:  reason,
		}

		err = repo.CreateCreditAdjustmentWithTx(ctx, &tx, adjustment)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)
	}

	testCases := []struct {
		name         string
		reasonPrefix string
		expectFound  bool
	}{
		{
			name:         "Find monthly top-up",
			reasonPrefix: "Monthly top-up",
			expectFound:  true,
		},
		{
			name:         "Find purchase",
			reasonPrefix: "Purchase",
			expectFound:  true,
		},
		{
			name:         "No match for non-existent prefix",
			reasonPrefix: "Refund",
			expectFound:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timestamp, err := repo.GetMostRecentCreditAdjustmentByReasonPrefix(ctx, tc.reasonPrefix)
			require.NoError(t, err)

			if tc.expectFound {
				require.NotNil(t, timestamp, "Should find a matching adjustment")
			} else {
				assert.Nil(t, timestamp, "Should not find a matching adjustment")
			}
		})
	}
}
