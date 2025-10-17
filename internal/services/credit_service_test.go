package services_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCreditService(
	t *testing.T,
) (services.CreditService, repositories.UserRepository, *repositories.CreditRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	creditRepo := repositories.NewCreditRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	service := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, userRepo)

	return *service, userRepo, creditRepo, transactor, cleanup
}

func TestCreditService_GetCreditBalance(t *testing.T) {
	testCases := []struct {
		name         string
		setupFn      func(userRepo repositories.UserRepository) string
		expectedFree int
		expectedPaid int
		wantErr      bool
	}{
		{
			name: "Valid user with mixed credits",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			expectedFree: 5,
			expectedPaid: 10,
			wantErr:      false,
		},
		{
			name: "Valid user with different credits",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 2,
					PaidCredits: 3,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			expectedFree: 2,
			expectedPaid: 3,
			wantErr:      false,
		},
		{
			name: "Non-existent user",
			setupFn: func(_ repositories.UserRepository) string {
				return gofakeit.UUID()
			},
			expectedFree: 0,
			expectedPaid: 0,
			wantErr:      true,
		},
		{
			name: "Empty user ID",
			setupFn: func(_ repositories.UserRepository) string {
				return ""
			},
			expectedFree: 0,
			expectedPaid: 0,
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, userRepo, _, _, cleanup := setupCreditService(t)
			defer cleanup()

			ctx := context.Background()
			userID := tc.setupFn(userRepo)

			freeCredits, paidCredits, err := svc.GetCreditBalance(ctx, userID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedFree, freeCredits)
				assert.Equal(t, tc.expectedPaid, paidCredits)
			}
		})
	}
}

func TestCreditService_AddCredits(t *testing.T) {
	testCases := []struct {
		name        string
		setupFn     func(userRepo repositories.UserRepository) string
		freeCredits int
		paidCredits int
		reason      string
		wantErr     bool
	}{
		{
			name: "Add free credits only",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 2,
					PaidCredits: 1,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test free credit addition",
			wantErr:     false,
		},
		{
			name: "Add paid credits only",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 2,
					PaidCredits: 1,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: 0,
			paidCredits: 7,
			reason:      "Test paid credit addition",
			wantErr:     false,
		},
		{
			name: "Cannot add both free and paid credits",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: 5,
			paidCredits: 5,
			reason:      "Invalid addition",
			wantErr:     true,
		},
		{
			name: "Cannot add negative free credits",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: -5,
			paidCredits: 0,
			reason:      "Invalid negative addition",
			wantErr:     true,
		},
		{
			name: "Cannot add negative paid credits",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: 0,
			paidCredits: -5,
			reason:      "Invalid negative addition",
			wantErr:     true,
		},
		{
			name: "Non-existent user",
			setupFn: func(_ repositories.UserRepository) string {
				return gofakeit.UUID()
			},
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test addition",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, userRepo, _, _, cleanup := setupCreditService(t)
			defer cleanup()

			ctx := context.Background()
			userID := tc.setupFn(userRepo)

			err := svc.AddCredits(ctx, userID, tc.freeCredits, tc.paidCredits, tc.reason)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreditService_DeductCreditForTeamStartWithTx(t *testing.T) {
	testCases := []struct {
		name       string
		setupFn    func(userRepo repositories.UserRepository) string
		teamID     string
		instanceID string
		wantErr    bool
	}{
		{
			name: "Valid team start deduction",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			teamID:     gofakeit.UUID(),
			instanceID: gofakeit.UUID(),
			wantErr:    false,
		},
		{
			name: "User with only paid credits",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 0,
					PaidCredits: 3,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			teamID:     gofakeit.UUID(),
			instanceID: gofakeit.UUID(),
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, userRepo, _, transactor, cleanup := setupCreditService(t)
			defer cleanup()

			ctx := context.Background()
			userID := tc.setupFn(userRepo)

			tx, err := transactor.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()

			err = svc.DeductCreditForTeamStartWithTx(ctx, tx, userID, tc.teamID, tc.instanceID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				txErr := tx.Commit()
				require.NoError(t, txErr)
			}
		})
	}
}

func TestCreditService_GetCreditAdjustments(t *testing.T) {
	testCases := []struct {
		name          string
		setupFn       func(svc services.CreditService, userRepo repositories.UserRepository) string
		filter        services.CreditAdjustmentFilter
		expectedCount int
		wantErr       bool
	}{
		{
			name: "Get all adjustments for user",
			setupFn: func(svc services.CreditService, userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				svc.AddCredits(context.Background(), user.ID, 5, 0, "Test adjustment 1")
				svc.AddCredits(context.Background(), user.ID, 3, 0, "Test adjustment 2")
				return user.ID
			},
			filter: services.CreditAdjustmentFilter{
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Get adjustments with limit",
			setupFn: func(svc services.CreditService, userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				svc.AddCredits(context.Background(), user.ID, 5, 0, "Test adjustment 1")
				svc.AddCredits(context.Background(), user.ID, 3, 0, "Test adjustment 2")
				return user.ID
			},
			filter: services.CreditAdjustmentFilter{
				Limit:  1,
				Offset: 0,
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Get adjustments for non-existent user",
			setupFn: func(_ services.CreditService, _ repositories.UserRepository) string {
				return gofakeit.UUID()
			},
			filter: services.CreditAdjustmentFilter{
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, userRepo, _, _, cleanup := setupCreditService(t)
			defer cleanup()

			ctx := context.Background()
			userID := tc.setupFn(svc, userRepo)
			tc.filter.UserID = userID

			adjustments, err := svc.GetCreditAdjustments(ctx, tc.filter)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, adjustments, tc.expectedCount)
			}
		})
	}
}

func TestCreditService_GetTeamStartLogsSummary(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(userRepo repositories.UserRepository) string
		groupBy string
		wantErr bool
	}{
		{
			name: "Valid daily summary",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			groupBy: "day",
			wantErr: false,
		},
		{
			name: "Valid weekly summary",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			groupBy: "week",
			wantErr: false,
		},
		{
			name: "Valid monthly summary",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			groupBy: "month",
			wantErr: false,
		},
		{
			name: "Invalid groupBy parameter",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			groupBy: "invalid",
			wantErr: true,
		},
		{
			name: "Default groupBy (empty)",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			groupBy: "",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, userRepo, _, _, cleanup := setupCreditService(t)
			defer cleanup()

			ctx := context.Background()
			userID := tc.setupFn(userRepo)

			filter := services.TeamStartLogFilter{
				UserID:  userID,
				GroupBy: tc.groupBy,
			}

			summary, err := svc.GetTeamStartLogsSummary(ctx, filter)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, summary)
			}
		})
	}
}

// Additional comprehensive tests for credit service

func TestCreditService_AddCredits_ValidationEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		setupFn     func(userRepo repositories.UserRepository) string
		freeCredits int
		paidCredits int
		reason      string
		wantErr     bool
		errorMsg    string
	}{
		{
			name: "Empty reason should fail",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: 5,
			paidCredits: 0,
			reason:      "",
			wantErr:     true,
			errorMsg:    "reason is required",
		},
		{
			name: "Zero credits for both should fail",
			setupFn: func(userRepo repositories.UserRepository) string {
				user := &models.User{
					ID:          gofakeit.UUID(),
					Email:       gofakeit.Email(),
					Name:        gofakeit.Name(),
					FreeCredits: 5,
					PaidCredits: 10,
					IsEducator:  false,
				}
				userRepo.Create(context.Background(), user)
				return user.ID
			},
			freeCredits: 0,
			paidCredits: 0,
			reason:      "Test",
			wantErr:     true,
			errorMsg:    "must add at least one credit",
		},
		{
			name: "Empty user ID should fail",
			setupFn: func(_ repositories.UserRepository) string {
				return ""
			},
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, userRepo, _, _, cleanup := setupCreditService(t)
			defer cleanup()

			ctx := context.Background()
			userID := tc.setupFn(userRepo)

			err := svc.AddCredits(ctx, userID, tc.freeCredits, tc.paidCredits, tc.reason)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreditService_DeductCreditForTeamStart_InsufficientCredits(t *testing.T) {
	svc, userRepo, creditRepo, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user (will get default 10 free credits)
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 10,
		PaidCredits: 0,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Use AddCreditsWithTx to set credits to exactly 0 (overriding DB defaults)
	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	err = creditRepo.AddCreditsWithTx(ctx, tx, user.ID, -10, 0) // Subtract the default 10
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Attempt to deduct credit with zero balance
	tx, err = transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	err = svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
	require.Error(t, err)
	assert.Equal(t, services.ErrInsufficientCredits, err)
}

func TestCreditService_DeductCreditForTeamStart_ConcurrentAccess(t *testing.T) {
	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with exactly 1 credit to test concurrent access
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 1,
		PaidCredits: 0,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Use channels to synchronize the goroutines for better race condition testing
	startSignal := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func(teamID string) {
			defer wg.Done()

			// Wait for start signal to ensure both goroutines start at the same time
			<-startSignal

			tx, txErr := transactor.BeginTx(ctx, nil)
			if txErr != nil {
				results <- txErr
				return
			}
			defer tx.Rollback()

			deductErr := svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, teamID, gofakeit.UUID())
			if deductErr == nil {
				deductErr = tx.Commit()
			}
			results <- deductErr
		}(gofakeit.UUID())
	}

	// Start both goroutines at the same time
	close(startSignal)
	wg.Wait()
	close(results)

	// Collect results
	var successCount, errorCount int
	var errList []error
	for err := range results {
		if err != nil {
			errorCount++
			errList = append(errList, err)
		} else {
			successCount++
		}
	}

	// Verify final state - user should have 0 credits if one operation succeeded
	finalFree, finalPaid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)
	finalTotal := finalFree + finalPaid

	// Either both failed (still has 1 credit) or one succeeded (has 0 credits)
	switch successCount {
	case 1:
		assert.Equal(t, 0, finalTotal, "User should have 0 credits if one operation succeeded")
		assert.Equal(t, 1, errorCount, "Exactly one operation should have failed")
		if len(errList) > 0 {
			// Accept either insufficient credits or database deadlock in concurrent scenarios
			failErr := errList[0]
			isValidError := errors.Is(failErr, services.ErrInsufficientCredits) ||
				strings.Contains(failErr.Error(), "database table is locked") ||
				strings.Contains(failErr.Error(), "database is deadlocked")
			assert.True(
				t,
				isValidError,
				"Failed attempt should be due to insufficient credits or database deadlock, got: %v",
				failErr,
			)
		}
	case 0:
		// Both operations failed - this is acceptable in concurrent scenarios
		assert.Equal(t, 1, finalTotal, "User should still have 1 credit if both operations failed")
		assert.Equal(t, 2, errorCount, "Both operations should have failed")
	case 2:
		// This indicates a race condition bug - both shouldn't succeed with only 1 credit
		t.Errorf("Race condition detected: both operations succeeded with only 1 credit available")
	}
}

func TestCreditService_GetCreditAdjustments_Pagination(t *testing.T) {
	svc, userRepo, _, _, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user and multiple credit adjustments
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 5,
		PaidCredits: 10,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	for i := range 5 {
		addErr := svc.AddCredits(ctx, user.ID, 1, 0, fmt.Sprintf("Test adjustment %d", i))
		require.NoError(t, addErr)
	}

	testCases := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "First page with limit 2",
			limit:         2,
			offset:        0,
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "Second page with limit 2",
			limit:         2,
			offset:        2,
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:          "Page beyond available records",
			limit:         10,
			offset:        100,
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := services.CreditAdjustmentFilter{
				UserID: user.ID,
				Limit:  tc.limit,
				Offset: tc.offset,
			}

			adjustments, getErr := svc.GetCreditAdjustments(ctx, filter)
			if tc.wantErr {
				require.Error(t, getErr)
			} else {
				require.NoError(t, getErr)
				assert.Len(t, adjustments, tc.expectedCount)
			}
		})
	}
}

func TestCreditService_TransactionRollback(t *testing.T) {
	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with credits
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 5,
		PaidCredits: 10,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Get initial credit balance
	initialFree, initialPaid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)

	// Start transaction and deduct credit
	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)

	err = svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
	require.NoError(t, err)

	// Rollback transaction
	err = tx.Rollback()
	require.NoError(t, err)

	// Verify credits were not deducted
	finalFree, finalPaid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)

	assert.Equal(t, initialFree, finalFree, "Free credits should be unchanged after rollback")
	assert.Equal(t, initialPaid, finalPaid, "Paid credits should be unchanged after rollback")
}
