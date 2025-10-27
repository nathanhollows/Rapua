package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCreditPurchaseRepo(t *testing.T) (*repositories.CreditPurchaseRepository, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	repo := repositories.NewCreditPurchaseRepository(dbc)

	return repo, dbc, cleanup
}

func createTestCreditPurchase(
	t *testing.T,
	db *bun.DB,
	userID string,
	credits int,
	status string,
) *models.CreditPurchase {
	t.Helper()

	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		UserID:          userID,
		Credits:         credits,
		AmountPaid:      models.CalculatePurchaseAmount(credits),
		StripeSessionID: gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: gofakeit.UUID(),
			Valid:  true,
		},
		Status: status,
	}

	_, err := db.NewInsert().
		Model(purchase).
		Exec(context.Background())
	require.NoError(t, err)

	return purchase
}

func TestCreditPurchaseRepo_Create(t *testing.T) {
	repo, _, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          gofakeit.UUID(),
		Credits:         10,
		AmountPaid:      350,
		StripeSessionID: gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: gofakeit.UUID(),
			Valid:  true,
		},
	}

	err := repo.Create(ctx, purchase)
	require.NoError(t, err)

	// Verify purchase was created
	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	assert.Equal(t, purchase.UserID, retrieved.UserID)
	assert.Equal(t, purchase.Credits, retrieved.Credits)
	assert.Equal(t, purchase.AmountPaid, retrieved.AmountPaid)
	assert.Equal(t, models.CreditPurchaseStatusPending, retrieved.Status)
}

func TestCreditPurchaseRepo_Create_ValidationErrors(t *testing.T) {
	repo, _, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	testCases := []struct {
		name        string
		purchase    *models.CreditPurchase
		expectedErr string
	}{
		{
			name: "Missing UserID",
			purchase: &models.CreditPurchase{
				ID:              gofakeit.UUID(),
				Credits:         10,
				AmountPaid:      350,
				StripeSessionID: gofakeit.UUID(),
			},
			expectedErr: "user_id is required",
		},
		{
			name: "Zero credits",
			purchase: &models.CreditPurchase{
				ID:              gofakeit.UUID(),
				UserID:          gofakeit.UUID(),
				Credits:         0,
				AmountPaid:      0,
				StripeSessionID: gofakeit.UUID(),
			},
			expectedErr: "credits must be greater than zero",
		},
		{
			name: "Negative amount",
			purchase: &models.CreditPurchase{
				ID:              gofakeit.UUID(),
				UserID:          gofakeit.UUID(),
				Credits:         10,
				AmountPaid:      -100,
				StripeSessionID: gofakeit.UUID(),
			},
			expectedErr: "amount_paid cannot be negative",
		},
		{
			name: "Missing StripeSessionID",
			purchase: &models.CreditPurchase{
				ID:         gofakeit.UUID(),
				UserID:     gofakeit.UUID(),
				Credits:    10,
				AmountPaid: 350,
			},
			expectedErr: "stripe_session_id is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.Create(ctx, tc.purchase)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestCreditPurchaseRepo_CreateWithTx(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          gofakeit.UUID(),
		Credits:         25,
		AmountPaid:      875,
		StripeSessionID: gofakeit.UUID(),
	}

	err = repo.CreateWithTx(ctx, &tx, purchase)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify purchase was created
	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	assert.Equal(t, purchase.Credits, retrieved.Credits)
}

func TestCreditPurchaseRepo_GetByStripeSessionID(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create test purchase
	sessionID := gofakeit.UUID()
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          gofakeit.UUID(),
		Credits:         15,
		AmountPaid:      525,
		StripeSessionID: sessionID,
		Status:          models.CreditPurchaseStatusPending,
	}

	_, err := db.NewInsert().Model(purchase).Exec(ctx)
	require.NoError(t, err)

	// Retrieve by session ID
	retrieved, err := repo.GetByStripeSessionID(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, purchase.ID, retrieved.ID)
	assert.Equal(t, purchase.UserID, retrieved.UserID)
}

func TestCreditPurchaseRepo_GetByStripeSessionID_NotFound(t *testing.T) {
	repo, _, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	_, err := repo.GetByStripeSessionID(ctx, "nonexistent-session")
	require.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestCreditPurchaseRepo_GetByID(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	purchase := createTestCreditPurchase(t, db, gofakeit.UUID(), 20, models.CreditPurchaseStatusCompleted)

	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, purchase.Credits, retrieved.Credits)
	assert.Equal(t, purchase.Status, retrieved.Status)
}

func TestCreditPurchaseRepo_GetByUserID(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create multiple purchases for the user
	purchase1 := createTestCreditPurchase(t, db, userID, 10, models.CreditPurchaseStatusCompleted)
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	purchase2 := createTestCreditPurchase(t, db, userID, 20, models.CreditPurchaseStatusPending)
	time.Sleep(10 * time.Millisecond)
	purchase3 := createTestCreditPurchase(t, db, userID, 30, models.CreditPurchaseStatusCompleted)

	// Create purchase for different user
	createTestCreditPurchase(t, db, gofakeit.UUID(), 50, models.CreditPurchaseStatusCompleted)

	// Get all purchases for user
	purchases, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.Len(t, purchases, 3)

	// Should be ordered by created_at DESC (most recent first)
	assert.Equal(t, purchase3.ID, purchases[0].ID)
	assert.Equal(t, purchase2.ID, purchases[1].ID)
	assert.Equal(t, purchase1.ID, purchases[2].ID)
}

func TestCreditPurchaseRepo_GetByUserIDWithPagination(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create 5 purchases
	for i := range 5 {
		createTestCreditPurchase(t, db, userID, (i+1)*10, models.CreditPurchaseStatusCompleted)
		time.Sleep(10 * time.Millisecond)
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
			name:          "Last page",
			limit:         2,
			offset:        4,
			expectedCount: 1,
		},
		{
			name:          "Beyond available records",
			limit:         10,
			offset:        10,
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			purchases, err := repo.GetByUserIDWithPagination(ctx, userID, tc.limit, tc.offset)
			require.NoError(t, err)
			assert.Len(t, purchases, tc.expectedCount)
		})
	}
}

func TestCreditPurchaseRepo_UpdateStatus(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	purchase := createTestCreditPurchase(t, db, gofakeit.UUID(), 15, models.CreditPurchaseStatusPending)

	// Update status to completed
	err := repo.UpdateStatus(ctx, purchase.ID, models.CreditPurchaseStatusCompleted)
	require.NoError(t, err)

	// Verify status was updated
	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	assert.Equal(t, models.CreditPurchaseStatusCompleted, retrieved.Status)
}

func TestCreditPurchaseRepo_UpdateStatusWithTx(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	purchase := createTestCreditPurchase(t, db, gofakeit.UUID(), 20, models.CreditPurchaseStatusPending)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.UpdateStatusWithTx(ctx, &tx, purchase.ID, models.CreditPurchaseStatusFailed)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify status was updated
	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	assert.Equal(t, models.CreditPurchaseStatusFailed, retrieved.Status)
}

func TestCreditPurchaseRepo_UpdateStripePaymentIDWithTx(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	purchase := createTestCreditPurchase(t, db, gofakeit.UUID(), 25, models.CreditPurchaseStatusPending)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	paymentID := "pi_" + gofakeit.UUID()
	err = repo.UpdateStripePaymentIDWithTx(ctx, &tx, purchase.ID, paymentID)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify payment ID was updated
	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	assert.True(t, retrieved.StripePaymentID.Valid)
	assert.Equal(t, paymentID, retrieved.StripePaymentID.String)
}

func TestCreditPurchaseRepo_UpdateReceiptURLWithTx(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	purchase := createTestCreditPurchase(t, db, gofakeit.UUID(), 30, models.CreditPurchaseStatusCompleted)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	receiptURL := "https://stripe.com/receipts/" + gofakeit.UUID()
	err = repo.UpdateReceiptURLWithTx(ctx, &tx, purchase.ID, receiptURL)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify receipt URL was updated
	retrieved, err := repo.GetByID(ctx, purchase.ID)
	require.NoError(t, err)
	assert.True(t, retrieved.ReceiptURL.Valid)
	assert.Equal(t, receiptURL, retrieved.ReceiptURL.String)
}

func TestCreditPurchaseRepo_DeleteByUserID(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	otherUserID := gofakeit.UUID()

	// Create purchases for both users
	purchase1 := createTestCreditPurchase(t, db, userID, 10, models.CreditPurchaseStatusCompleted)
	purchase2 := createTestCreditPurchase(t, db, userID, 20, models.CreditPurchaseStatusPending)
	otherPurchase := createTestCreditPurchase(t, db, otherUserID, 30, models.CreditPurchaseStatusCompleted)

	// Delete purchases for first user
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.DeleteByUserID(ctx, &tx, userID)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify user's purchases were deleted
	_, err = repo.GetByID(ctx, purchase1.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)

	_, err = repo.GetByID(ctx, purchase2.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)

	// Verify other user's purchase still exists
	otherRetrieved, err := repo.GetByID(ctx, otherPurchase.ID)
	require.NoError(t, err)
	assert.NotNil(t, otherRetrieved)
	assert.Equal(t, otherPurchase.ID, otherRetrieved.ID)
}

func TestCreditPurchaseRepo_DeleteByUserID_EmptyResult(t *testing.T) {
	repo, db, cleanup := setupCreditPurchaseRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Delete for non-existent user should not error
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = repo.DeleteByUserID(ctx, &tx, "nonexistent-user")
	require.NoError(t, err)
}
