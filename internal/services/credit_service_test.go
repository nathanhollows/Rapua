package services_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCreditService(t *testing.T) (services.CreditService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	creditRepo := repositories.NewCreditRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	service := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, userRepo)

	// Create test users
	ctx := context.Background()
	
	user1 := &models.User{
		ID:          "user-credit-1",
		Email:       "user1@example.com",
		Name:        "Test User 1",
		FreeCredits: 5,
		PaidCredits: 10,
		IsEducator:  false,
	}
	userRepo.Create(ctx, user1)

	user2 := &models.User{
		ID:          "user-credit-2",
		Email:       "user2@example.com",
		Name:        "Test User 2",
		FreeCredits: 0,
		PaidCredits: 3,
		IsEducator:  false,
	}
	userRepo.Create(ctx, user2)

	return *service, cleanup
}

func TestCreditService_GetCreditBalance(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()
	dbc, _ := setupDB(t)
	userRepo := repositories.NewUserRepository(dbc)

	// Create fresh test users for this specific test
	freshUser1 := &models.User{
		ID:          "fresh-user-1",
		Email:       "fresh1@example.com",
		Name:        "Fresh User 1",
		FreeCredits: 5,
		PaidCredits: 10,
		IsEducator:  false,
	}
	userRepo.Create(ctx, freshUser1)

	freshUser2 := &models.User{
		ID:          "fresh-user-2",
		Email:       "fresh2@example.com",
		Name:        "Fresh User 2",
		FreeCredits: 2, // Use non-default value
		PaidCredits: 3,
		IsEducator:  false,
	}
	userRepo.Create(ctx, freshUser2)

	tests := []struct {
		name            string
		userID          string
		expectedFree    int
		expectedPaid    int
		wantErr         bool
	}{
		{
			name:         "Valid user with mixed credits",
			userID:       "fresh-user-1",
			expectedFree: 5,
			expectedPaid: 10,
			wantErr:      false,
		},
		{
			name:         "Valid user with different credits",
			userID:       "fresh-user-2",
			expectedFree: 2,
			expectedPaid: 3,
			wantErr:      false,
		},
		{
			name:         "Non-existent user",
			userID:       "non-existent",
			expectedFree: 0,
			expectedPaid: 0,
			wantErr:      true,
		},
		{
			name:         "Empty user ID",
			userID:       "",
			expectedFree: 0,
			expectedPaid: 0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freeCredits, paidCredits, err := service.GetCreditBalance(ctx, tt.userID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if freeCredits != tt.expectedFree || paidCredits != tt.expectedPaid {
					t.Logf("User %s: expected free=%d, paid=%d; got free=%d, paid=%d",
						tt.userID, tt.expectedFree, tt.expectedPaid, freeCredits, paidCredits)
				}
				assert.Equal(t, tt.expectedFree, freeCredits)
				assert.Equal(t, tt.expectedPaid, paidCredits)
			}
		})
	}
}

func TestCreditService_AddCredits(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a fresh user for this test to avoid state interference
	freshUser := &models.User{
		ID:          "user-add-credits-test",
		Email:       "addtest@example.com",
		Name:        "Add Credits Test User",
		FreeCredits: 2,
		PaidCredits: 1,
		IsEducator:  false,
	}
	dbc, _ := setupDB(t)
	userRepo := repositories.NewUserRepository(dbc)
	userRepo.Create(ctx, freshUser)

	tests := []struct {
		name        string
		userID      string
		freeCredits int
		paidCredits int
		reason      string
		wantErr     bool
	}{
		{
			name:        "Add free credits only",
			userID:      "user-add-credits-test",
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test free credit addition",
			wantErr:     false,
		},
		{
			name:        "Add paid credits only",
			userID:      "user-add-credits-test",
			freeCredits: 0,
			paidCredits: 7,
			reason:      "Test paid credit addition",
			wantErr:     false,
		},
		{
			name:        "Cannot add both free and paid credits",
			userID:      "user-credit-1",
			freeCredits: 5,
			paidCredits: 5,
			reason:      "Invalid addition",
			wantErr:     true,
		},
		{
			name:        "Cannot add negative free credits",
			userID:      "user-credit-1",
			freeCredits: -5,
			paidCredits: 0,
			reason:      "Invalid negative addition",
			wantErr:     true,
		},
		{
			name:        "Cannot add negative paid credits",
			userID:      "user-credit-1",
			freeCredits: 0,
			paidCredits: -5,
			reason:      "Invalid negative addition",
			wantErr:     true,
		},
		{
			name:        "Non-existent user",
			userID:      "non-existent",
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test addition",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.AddCredits(ctx, tt.userID, tt.freeCredits, tt.paidCredits, tt.reason)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreditService_DeductCredits(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name         string
		userID       string
		creditsToDeduct int
		expectedSuccess bool
		wantErr      bool
	}{
		{
			name:            "Deduct from free credits",
			userID:          "user-credit-1",
			creditsToDeduct: 3,
			expectedSuccess: true,
			wantErr:         false,
		},
		{
			name:            "Deduct more than free, use paid",
			userID:          "user-credit-1", 
			creditsToDeduct: 8,
			expectedSuccess: true,
			wantErr:         false,
		},
		{
			name:            "Insufficient total credits",
			userID:          "user-credit-1",
			creditsToDeduct: 20,
			expectedSuccess: false,
			wantErr:         false,
		},
		{
			name:            "Deduct from paid only user",
			userID:          "user-credit-2",
			creditsToDeduct: 2,
			expectedSuccess: true,
			wantErr:         false,
		},
		{
			name:            "Non-existent user",
			userID:          "non-existent",
			creditsToDeduct: 1,
			expectedSuccess: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, err := service.DeductCredits(ctx, tt.userID, tt.creditsToDeduct)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSuccess, success)
			}
		})
	}
}

func TestCreditService_DeductCreditForTeamStartWithTx(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()
	transactor := db.NewTransactor(setupDBConnection(t))

	tests := []struct {
		name       string
		userID     string
		teamID     string
		instanceID string
		wantErr    bool
	}{
		{
			name:       "Valid team start deduction",
			userID:     "user-credit-1",
			teamID:     "team-123",
			instanceID: "instance-456",
			wantErr:    false,
		},
		{
			name:       "User with only paid credits",
			userID:     "user-credit-2",
			teamID:     "team-124",
			instanceID: "instance-456",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := transactor.BeginTx(ctx, nil)
			assert.NoError(t, err)
			defer tx.Rollback()

			err = service.DeductCreditForTeamStartWithTx(ctx, tx, tt.userID, tt.teamID, tt.instanceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreditService_GetCreditAdjustments(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Add some credits to create adjustment records
	err := service.AddCredits(ctx, "user-credit-1", 5, 0, "Test adjustment 1")
	assert.NoError(t, err)
	
	err = service.AddCredits(ctx, "user-credit-1", 3, 0, "Test adjustment 2")
	assert.NoError(t, err)

	tests := []struct {
		name           string
		filter         services.CreditAdjustmentFilter
		expectedCount  int
		wantErr        bool
	}{
		{
			name: "Get all adjustments for user",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Get adjustments with limit",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  1,
				Offset: 0,
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Get adjustments for non-existent user",
			filter: services.CreditAdjustmentFilter{
				UserID: "non-existent",
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjustments, err := service.GetCreditAdjustments(ctx, tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, adjustments, tt.expectedCount)
			}
		})
	}
}

func TestCreditService_GetTeamStartLogsSummary(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name    string
		filter  services.TeamStartLogFilter
		wantErr bool
	}{
		{
			name: "Valid daily summary",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "day",
			},
			wantErr: false,
		},
		{
			name: "Valid weekly summary",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "week",
			},
			wantErr: false,
		},
		{
			name: "Valid monthly summary",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "month",
			},
			wantErr: false,
		},
		{
			name: "Invalid groupBy parameter",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "invalid",
			},
			wantErr: true,
		},
		{
			name: "Default groupBy (empty)",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := service.GetTeamStartLogsSummary(ctx, tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, summary)
			}
		})
	}
}

// Helper function to get DB connection for transaction tests
func setupDBConnection(t *testing.T) *bun.DB {
	t.Helper()
	dbc, _ := setupDB(t)
	return dbc
}

// Additional comprehensive tests for credit service

func TestCreditService_AddCredits_ValidationEdgeCases(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name        string
		userID      string
		freeCredits int
		paidCredits int
		reason      string
		wantErr     bool
		errorMsg    string
	}{
		{
			name:        "Empty reason should fail",
			userID:      "user-credit-1",
			freeCredits: 5,
			paidCredits: 0,
			reason:      "",
			wantErr:     true,
			errorMsg:    "reason is required",
		},
		{
			name:        "Zero credits for both should fail",
			userID:      "user-credit-1",
			freeCredits: 0,
			paidCredits: 0,
			reason:      "Test",
			wantErr:     true,
			errorMsg:    "must add at least one credit",
		},
		{
			name:        "Empty user ID should fail",
			userID:      "",
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.AddCredits(ctx, tt.userID, tt.freeCredits, tt.paidCredits, tt.reason)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreditService_DeductCreditForTeamStart_InsufficientCredits(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()
	transactor := db.NewTransactor(setupDBConnection(t))

	// Create user with zero credits
	dbc, _ := setupDB(t)
	userRepo := repositories.NewUserRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	zeroCreditsUser := &models.User{
		ID:          "user-zero-credits",
		Email:       "zero@example.com",
		Name:        "Zero Credits User",
		FreeCredits: 1, // Create with some credits first
		PaidCredits: 0,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, zeroCreditsUser)
	require.NoError(t, err)

	// Now manually set credits to zero to override database defaults
	err = creditRepo.UpdateCredits(ctx, "user-zero-credits", 0, 0)
	require.NoError(t, err)

	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = service.DeductCreditForTeamStartWithTx(ctx, tx, "user-zero-credits", "team-123", "instance-456")
	assert.Error(t, err)
	assert.Equal(t, services.ErrInsufficientCredits, err)
}

func TestCreditService_DeductCreditForTeamStart_ConcurrentAccess(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()
	transactor := db.NewTransactor(setupDBConnection(t))

	// Create user with exactly 1 credit
	dbc, _ := setupDB(t)
	userRepo := repositories.NewUserRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	oneCreditsUser := &models.User{
		ID:          "user-concurrent",
		Email:       "concurrent@example.com",
		Name:        "Concurrent Test User",
		FreeCredits: 2,
		PaidCredits: 0,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, oneCreditsUser)
	require.NoError(t, err)

	// Set exactly 1 credit to test concurrent access
	err = creditRepo.UpdateCredits(ctx, "user-concurrent", 1, 0)
	require.NoError(t, err)

	// Use channels to synchronize the goroutines for better race condition testing
	startSignal := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(teamID string) {
			defer wg.Done()

			// Wait for start signal to ensure both goroutines start at the same time
			<-startSignal

			tx, err := transactor.BeginTx(ctx, nil)
			if err != nil {
				results <- err
				return
			}
			defer tx.Rollback()

			err = service.DeductCreditForTeamStartWithTx(ctx, tx, "user-concurrent", teamID, "instance-456")
			if err == nil {
				err = tx.Commit()
			}
			results <- err
		}(fmt.Sprintf("team-concurrent-%d", i))
	}

	// Start both goroutines at the same time
	close(startSignal)
	wg.Wait()
	close(results)

	// Collect results
	var successCount, errorCount int
	var errors []error
	for err := range results {
		if err != nil {
			errorCount++
			errors = append(errors, err)
		} else {
			successCount++
		}
	}

	// Verify final state - user should have 0 credits if one operation succeeded
	finalFree, finalPaid, err := service.GetCreditBalance(ctx, "user-concurrent")
	require.NoError(t, err)
	finalTotal := finalFree + finalPaid

	// Either both failed (still has 1 credit) or one succeeded (has 0 credits)
	if successCount == 1 {
		assert.Equal(t, 0, finalTotal, "User should have 0 credits if one operation succeeded")
		assert.Equal(t, 1, errorCount, "Exactly one operation should have failed")
		if len(errors) > 0 {
			// Accept either insufficient credits or database deadlock in concurrent scenarios
			err := errors[0]
			isValidError := err == services.ErrInsufficientCredits ||
				strings.Contains(err.Error(), "database table is locked") ||
				strings.Contains(err.Error(), "database is deadlocked")
			assert.True(t, isValidError, "Failed attempt should be due to insufficient credits or database deadlock, got: %v", err)
		}
	} else if successCount == 0 {
		// Both operations failed - this is acceptable in concurrent scenarios
		assert.Equal(t, 1, finalTotal, "User should still have 1 credit if both operations failed")
		assert.Equal(t, 2, errorCount, "Both operations should have failed")
	} else if successCount == 2 {
		// This indicates a race condition bug - both shouldn't succeed with only 1 credit
		t.Errorf("Race condition detected: both operations succeeded with only 1 credit available")
	}
}

func TestCreditService_DeductCredits_EdgeCases(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name            string
		userID          string
		creditsToDeduct int
		wantSuccess     bool
		wantErr         bool
	}{
		{
			name:            "Deduct zero credits should succeed",
			userID:          "user-credit-1",
			creditsToDeduct: 0,
			wantSuccess:     true,
			wantErr:         false,
		},
		{
			name:            "Deduct negative credits should fail",
			userID:          "user-credit-1",
			creditsToDeduct: -5,
			wantSuccess:     false,
			wantErr:         false, // Returns false but no error
		},
		{
			name:            "Deduct from empty user ID should error",
			userID:          "",
			creditsToDeduct: 1,
			wantSuccess:     false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, err := service.DeductCredits(ctx, tt.userID, tt.creditsToDeduct)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantSuccess, success)
			}
		})
	}
}

func TestCreditService_GetCreditAdjustments_Pagination(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple credit adjustments
	for i := 0; i < 5; i++ {
		err := service.AddCredits(ctx, "user-credit-1", 1, 0, fmt.Sprintf("Test adjustment %d", i))
		assert.NoError(t, err)
	}

	tests := []struct {
		name           string
		filter         services.CreditAdjustmentFilter
		expectedCount  int
		wantErr        bool
	}{
		{
			name: "First page with limit 2",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  2,
				Offset: 0,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Second page with limit 2",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  2,
				Offset: 2,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Page beyond available records",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  10,
				Offset: 100,
			},
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjustments, err := service.GetCreditAdjustments(ctx, tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(adjustments))
			}
		})
	}
}

func TestCreditService_TransactionRollback(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()
	transactor := db.NewTransactor(setupDBConnection(t))

	// Get initial credit balance
	initialFree, initialPaid, err := service.GetCreditBalance(ctx, "user-credit-1")
	require.NoError(t, err)

	// Start transaction and deduct credit
	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)

	err = service.DeductCreditForTeamStartWithTx(ctx, tx, "user-credit-1", "team-rollback", "instance-456")
	assert.NoError(t, err)

	// Rollback transaction
	err = tx.Rollback()
	assert.NoError(t, err)

	// Verify credits were not deducted
	finalFree, finalPaid, err := service.GetCreditBalance(ctx, "user-credit-1")
	require.NoError(t, err)

	assert.Equal(t, initialFree, finalFree, "Free credits should be unchanged after rollback")
	assert.Equal(t, initialPaid, finalPaid, "Paid credits should be unchanged after rollback")
}