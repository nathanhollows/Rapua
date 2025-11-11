package services_test

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStalePurchaseCleanupService(
	t *testing.T,
) (services.StalePurchaseCleanupService, *repositories.CreditPurchaseRepository, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)
	logger := slog.Default()

	purchaseRepo := repositories.NewCreditPurchaseRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	service := services.NewStalePurchaseCleanupService(transactor, logger)

	// Create a test user for purchase records
	ctx := context.Background()
	testUser := &models.User{
		ID:                 "user-test-cleanup",
		Email:              "cleanup@example.com",
		Name:               "Cleanup Test User",
		FreeCredits:        10,
		PaidCredits:        0,
		MonthlyCreditLimit: 10,
	}
	userRepo.Create(ctx, testUser)

	return *service, purchaseRepo, cleanup
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_FirstRun(t *testing.T) {
	service, _, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Run the cleanup process
	err := service.CleanupStalePurchases(ctx)
	require.NoError(t, err)
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_DeletesStalePending(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a stale pending purchase (8 days old)
	stalePurchase := &models.CreditPurchase{
		ID:              "purchase-stale-pending",
		UserID:          "user-test-cleanup",
		Credits:         100,
		AmountPaid:      1000,
		StripeSessionID: "sess_stale_pending",
		Status:          models.CreditPurchaseStatusPending,
		CreatedAt:       time.Now().AddDate(0, 0, -8),
		UpdatedAt:       time.Now().AddDate(0, 0, -8),
	}
	err := purchaseRepo.Create(ctx, stalePurchase)
	require.NoError(t, err)

	// Run the cleanup process
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify the stale purchase was deleted
	_, err = purchaseRepo.GetByID(ctx, "purchase-stale-pending")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows, "Stale pending purchase should be deleted")
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_DeletesStaleFailed(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a stale failed purchase (10 days old)
	stalePurchase := &models.CreditPurchase{
		ID:              "purchase-stale-failed",
		UserID:          "user-test-cleanup",
		Credits:         100,
		AmountPaid:      1000,
		StripeSessionID: "sess_stale_failed",
		Status:          models.CreditPurchaseStatusFailed,
		CreatedAt:       time.Now().AddDate(0, 0, -10),
		UpdatedAt:       time.Now().AddDate(0, 0, -10),
	}
	err := purchaseRepo.Create(ctx, stalePurchase)
	require.NoError(t, err)

	// Run the cleanup process
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify the stale purchase was deleted
	_, err = purchaseRepo.GetByID(ctx, "purchase-stale-failed")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows, "Stale failed purchase should be deleted")
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_PreservesRecentPending(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a recent pending purchase (2 days old)
	recentPurchase := &models.CreditPurchase{
		ID:              "purchase-recent-pending",
		UserID:          "user-test-cleanup",
		Credits:         100,
		AmountPaid:      1000,
		StripeSessionID: "sess_recent_pending",
		Status:          models.CreditPurchaseStatusPending,
		CreatedAt:       time.Now().AddDate(0, 0, -2),
		UpdatedAt:       time.Now().AddDate(0, 0, -2),
	}
	err := purchaseRepo.Create(ctx, recentPurchase)
	require.NoError(t, err)

	// Run the cleanup process
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify the recent purchase was NOT deleted
	retrieved, err := purchaseRepo.GetByID(ctx, "purchase-recent-pending")
	require.NoError(t, err, "Recent pending purchase should be preserved")
	assert.NotNil(t, retrieved)
	assert.Equal(t, models.CreditPurchaseStatusPending, retrieved.Status)
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_PreservesCompletedPurchases(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create an old completed purchase (30 days old)
	completedPurchase := &models.CreditPurchase{
		ID:              "purchase-old-completed",
		UserID:          "user-test-cleanup",
		Credits:         100,
		AmountPaid:      1000,
		StripeSessionID: "sess_old_completed",
		Status:          models.CreditPurchaseStatusCompleted,
		CreatedAt:       time.Now().AddDate(0, 0, -30),
		UpdatedAt:       time.Now().AddDate(0, 0, -30),
	}
	err := purchaseRepo.Create(ctx, completedPurchase)
	require.NoError(t, err)

	// Run the cleanup process
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify the completed purchase was NOT deleted
	retrieved, err := purchaseRepo.GetByID(ctx, "purchase-old-completed")
	require.NoError(t, err, "Old completed purchase should be preserved")
	assert.NotNil(t, retrieved)
	assert.Equal(t, models.CreditPurchaseStatusCompleted, retrieved.Status)
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_MixedScenario(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple purchases with different statuses and ages
	purchases := []*models.CreditPurchase{
		// Should be deleted
		{
			ID:              "purchase-stale-pending-1",
			UserID:          "user-test-cleanup",
			Credits:         100,
			AmountPaid:      1000,
			StripeSessionID: "sess_stale_pending_1",
			Status:          models.CreditPurchaseStatusPending,
			CreatedAt:       time.Now().AddDate(0, 0, -8),
			UpdatedAt:       time.Now().AddDate(0, 0, -8),
		},
		// Should be deleted
		{
			ID:              "purchase-stale-failed-1",
			UserID:          "user-test-cleanup",
			Credits:         200,
			AmountPaid:      2000,
			StripeSessionID: "sess_stale_failed_1",
			Status:          models.CreditPurchaseStatusFailed,
			CreatedAt:       time.Now().AddDate(0, 0, -15),
			UpdatedAt:       time.Now().AddDate(0, 0, -15),
		},
		// Should be preserved (recent)
		{
			ID:              "purchase-recent-pending-1",
			UserID:          "user-test-cleanup",
			Credits:         150,
			AmountPaid:      1500,
			StripeSessionID: "sess_recent_pending_1",
			Status:          models.CreditPurchaseStatusPending,
			CreatedAt:       time.Now().AddDate(0, 0, -3),
			UpdatedAt:       time.Now().AddDate(0, 0, -3),
		},
		// Should be preserved (completed)
		{
			ID:              "purchase-old-completed-1",
			UserID:          "user-test-cleanup",
			Credits:         300,
			AmountPaid:      3000,
			StripeSessionID: "sess_old_completed_1",
			Status:          models.CreditPurchaseStatusCompleted,
			CreatedAt:       time.Now().AddDate(0, 0, -20),
			UpdatedAt:       time.Now().AddDate(0, 0, -20),
		},
	}

	for _, p := range purchases {
		err := purchaseRepo.Create(ctx, p)
		require.NoError(t, err)
	}

	// Run the cleanup process
	err := service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify stale purchases were deleted
	_, err = purchaseRepo.GetByID(ctx, "purchase-stale-pending-1")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows, "Stale pending purchase should be deleted")

	_, err = purchaseRepo.GetByID(ctx, "purchase-stale-failed-1")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows, "Stale failed purchase should be deleted")

	// Verify preserved purchases still exist
	retrieved, err := purchaseRepo.GetByID(ctx, "purchase-recent-pending-1")
	require.NoError(t, err, "Recent pending purchase should be preserved")
	assert.NotNil(t, retrieved)

	retrieved, err = purchaseRepo.GetByID(ctx, "purchase-old-completed-1")
	require.NoError(t, err, "Old completed purchase should be preserved")
	assert.NotNil(t, retrieved)
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_EdgeCases(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a purchase just under 7 days old (6.5 days - boundary case, should be preserved)
	boundaryPreserved := &models.CreditPurchase{
		ID:              "purchase-boundary-preserved",
		UserID:          "user-test-cleanup",
		Credits:         100,
		AmountPaid:      1000,
		StripeSessionID: "sess_boundary_preserved",
		Status:          models.CreditPurchaseStatusPending,
		CreatedAt:       time.Now().Add(-6*24*time.Hour - 12*time.Hour),
		UpdatedAt:       time.Now().Add(-6*24*time.Hour - 12*time.Hour),
	}
	err := purchaseRepo.Create(ctx, boundaryPreserved)
	require.NoError(t, err)

	// Create a purchase just over 7 days old (7.5 days - boundary case, should be deleted)
	boundaryDeleted := &models.CreditPurchase{
		ID:              "purchase-boundary-deleted",
		UserID:          "user-test-cleanup",
		Credits:         200,
		AmountPaid:      2000,
		StripeSessionID: "sess_boundary_deleted",
		Status:          models.CreditPurchaseStatusPending,
		CreatedAt:       time.Now().Add(-7*24*time.Hour - 12*time.Hour),
		UpdatedAt:       time.Now().Add(-7*24*time.Hour - 12*time.Hour),
	}
	err = purchaseRepo.Create(ctx, boundaryDeleted)
	require.NoError(t, err)

	// Run the cleanup process
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify the purchase just under 7 days old is preserved
	retrieved, err := purchaseRepo.GetByID(ctx, "purchase-boundary-preserved")
	require.NoError(t, err)
	assert.NotNil(t, retrieved, "Purchase just under 7 days old should be preserved")

	// Verify the purchase just over 7 days old is deleted
	_, err = purchaseRepo.GetByID(ctx, "purchase-boundary-deleted")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows, "Purchase just over 7 days old should be deleted")
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_IdempotencyCheck(t *testing.T) {
	service, purchaseRepo, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a stale purchase
	stalePurchase := &models.CreditPurchase{
		ID:              "purchase-stale-idempotent",
		UserID:          "user-test-cleanup",
		Credits:         100,
		AmountPaid:      1000,
		StripeSessionID: "sess_stale_idempotent",
		Status:          models.CreditPurchaseStatusPending,
		CreatedAt:       time.Now().AddDate(0, 0, -10),
		UpdatedAt:       time.Now().AddDate(0, 0, -10),
	}
	err := purchaseRepo.Create(ctx, stalePurchase)
	require.NoError(t, err)

	// Run cleanup first time
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err)

	// Verify purchase was deleted
	_, err = purchaseRepo.GetByID(ctx, "purchase-stale-idempotent")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows, "Stale purchase should be deleted after first run")

	// Run cleanup second time (should be idempotent, no errors)
	err = service.CleanupStalePurchases(ctx)
	require.NoError(t, err, "Second cleanup run should be idempotent")
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_EmptyDatabase(t *testing.T) {
	service, _, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	ctx := context.Background()

	// Run cleanup with no purchases in database
	err := service.CleanupStalePurchases(ctx)
	require.NoError(t, err, "Cleanup should handle empty database gracefully")
}

func TestStalePurchaseCleanupService_CleanupStalePurchases_ContextCancellation(t *testing.T) {
	service, _, cleanup := setupStalePurchaseCleanupService(t)
	defer cleanup()

	// Create a context that will be cancelled immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// The service should handle context cancellation gracefully
	err := service.CleanupStalePurchases(ctx)

	// Should either succeed (if it completed before cancellation) or handle cancellation gracefully
	if err != nil {
		// If there's an error, it should be related to context cancellation
		t.Logf("Service handled context cancellation: %v", err)
	}
}
