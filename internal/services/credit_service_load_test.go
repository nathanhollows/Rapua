package services_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreditService_ConcurrentTeamStarts_100Users tests 100 concurrent team starts.
func TestCreditService_ConcurrentTeamStarts_100Users(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	const numUsers = 100

	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with numUsers credits
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 50,
		PaidCredits: 50,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Track results
	var successCount, failureCount atomic.Int32
	var wg sync.WaitGroup
	startSignal := make(chan struct{})
	results := make(chan error, numUsers)

	// Start numUsers concurrent team starts
	for range numUsers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait for start signal
			<-startSignal

			tx, txErr := transactor.BeginTx(ctx, nil)
			if txErr != nil {
				results <- txErr
				failureCount.Add(1)
				return
			}
			defer tx.Rollback()

			teamID := gofakeit.UUID()
			deductErr := svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, teamID, gofakeit.UUID())
			if deductErr != nil {
				results <- deductErr
				failureCount.Add(1)
				return
			}

			commitErr := tx.Commit()
			if commitErr != nil {
				results <- commitErr
				failureCount.Add(1)
				return
			}

			successCount.Add(1)
			results <- nil
		}()
	}

	// Start all goroutines simultaneously
	close(startSignal)
	wg.Wait()
	close(results)

	// Verify results
	finalFree, finalPaid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)
	finalTotal := finalFree + finalPaid

	t.Logf("Started with 100 credits")
	t.Logf("Successful operations: %d", successCount.Load())
	t.Logf("Failed operations: %d", failureCount.Load())
	t.Logf("Final credits: %d", finalTotal)

	// All numUsers operations should succeed
	assert.Equal(t, int32(numUsers), successCount.Load(), "All 100 operations should succeed")
	assert.Equal(t, 0, finalTotal, "User should have 0 credits after 100 team starts")
}

// TestCreditService_PerformanceUnder50ms tests that team starts complete within 50ms.
func TestCreditService_PerformanceUnder50ms(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	const numOperations = 100

	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with enough credits
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: numOperations,
		PaidCredits: 0,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Run numOperations,sequential operations and measure each
	var totalDuration time.Duration
	var maxDuration time.Duration
	var operationsOver50ms int

	for range numOperations {
		tx, txErr := transactor.BeginTx(ctx, nil)
		require.NoError(t, txErr)

		start := time.Now()
		deductErr := svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
		elapsed := time.Since(start)

		require.NoError(t, deductErr)
		commitErr := tx.Commit()
		require.NoError(t, commitErr)

		totalDuration += elapsed
		if elapsed > maxDuration {
			maxDuration = elapsed
		}
		if elapsed > 50*time.Millisecond {
			operationsOver50ms++
		}
	}

	avgDuration := totalDuration / numOperations

	t.Logf("Average operation time: %v", avgDuration)
	t.Logf("Maximum operation time: %v", maxDuration)
	t.Logf("Operations over 50ms: %d/%d", operationsOver50ms, numOperations)

	// Assert performance requirements
	assert.Less(t, avgDuration, 50*time.Millisecond, "Average operation should be under 50ms")
	assert.Less(t, operationsOver50ms, 10, "Less than 10% of operations should exceed 50ms")
}

// TestCreditService_EdgeCase_ExactlyZeroCredits tests behavior with exactly 0 credits.
func TestCreditService_EdgeCase_ExactlyZeroCredits(t *testing.T) {
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

	// Try to start a team with 0 credits
	tx, err = transactor.BeginTx(ctx, nil)
	require.NoError(t, err)

	err = svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
	if err == nil {
		_ = tx.Commit()
	} else {
		_ = tx.Rollback()
	}

	require.Error(t, err, "Should fail with 0 credits")
	assert.Equal(t, services.ErrInsufficientCredits, err)

	// Verify credits unchanged
	finalFree, finalPaid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, finalFree+finalPaid, "Credits should remain at 0")
}

// TestCreditService_ConcurrentMixedOperations verifies that concurrent credit additions
// and deductions produce a consistent final balance.
func TestCreditService_ConcurrentMixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a user starting with 100 free credits.
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 100,
		PaidCredits: 0,
		IsEducator:  false,
	}
	require.NoError(t, userRepo.Create(ctx, user))

	const numOps = 25

	var (
		addSuccess    atomic.Int32
		deductSuccess atomic.Int32
		wg            sync.WaitGroup
		startSignal   = make(chan struct{})
	)

	// Concurrent additions
	for i := range numOps {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-startSignal

			addErr := svc.AddCredits(ctx, user.ID, 1, 0, fmt.Sprintf("Add %d", n))
			if addErr == nil {
				addSuccess.Add(1)
			} else {
				t.Logf("Add failed (op %d): %v", n, addErr)
			}
		}(i)
	}

	// Concurrent deductions (team starts)
	for i := range numOps {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-startSignal

			tx, txErr := transactor.BeginTx(ctx, nil)
			if txErr != nil {
				t.Logf("Failed to begin transaction (op %d): %v", n, txErr)
				return
			}
			defer tx.Rollback()

			deductErr := svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
			if deductErr != nil {
				t.Logf("Deduct failed (op %d): %v", n, deductErr)
				return
			}

			if commitErr := tx.Commit(); commitErr == nil {
				deductSuccess.Add(1)
			} else {
				t.Logf("Commit failed (op %d): %v", n, commitErr)
			}
		}(i)
	}

	// Start all goroutines at once
	close(startSignal)
	wg.Wait()

	// Verify final balance
	free, paid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)

	finalTotal := free + paid
	expectedTotal := 100 + int(addSuccess.Load()) - int(deductSuccess.Load())

	t.Logf("Add successes: %d/%d", addSuccess.Load(), numOps)
	t.Logf("Deduct successes: %d/%d", deductSuccess.Load(), numOps)
	t.Logf("Final total credits: %d (expected %d)", finalTotal, expectedTotal)

	assert.Equal(t, expectedTotal, finalTotal, "final credit total mismatch")
}

// TestCreditService_StressTest_250Operations tests system stability under heavy load.
func TestCreditService_StressTest_250Operations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with plenty of credits
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 150,
		PaidCredits: 100,
		IsEducator:  false,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	var successCount, failureCount atomic.Int32
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 50) // Limit to 50 concurrent goroutines

	start := time.Now()

	for range 250 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			tx, txErr := transactor.BeginTx(ctx, nil)
			if txErr != nil {
				failureCount.Add(1)
				return
			}
			defer tx.Rollback()

			deductErr := svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
			if deductErr != nil {
				failureCount.Add(1)
				return
			}

			commitErr := tx.Commit()
			if commitErr != nil {
				failureCount.Add(1)
				return
			}

			successCount.Add(1)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	finalFree, finalPaid, err := svc.GetCreditBalance(ctx, user.ID)
	require.NoError(t, err)
	finalTotal := finalFree + finalPaid

	t.Logf("250 operations completed in %v", elapsed)
	t.Logf("Success: %d, Failures: %d", successCount.Load(), failureCount.Load())
	t.Logf("Final credits: %d (expected: 0)", finalTotal)
	t.Logf("Throughput: %.2f ops/sec", float64(250)/elapsed.Seconds())

	assert.Equal(t, int32(250), successCount.Load(), "All 250 operations should succeed")
	assert.Equal(t, 0, finalTotal, "Final credits should be 0")
}

// TestCreditService_RaceCondition_SingleCredit tests race conditions with exactly 1 credit.
func TestCreditService_RaceCondition_SingleCredit(t *testing.T) {
	svc, userRepo, _, transactor, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Run test multiple times to catch race conditions
	for attempt := range 5 {
		// Create user with exactly 1 credit
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

		// 10 goroutines trying to deduct the single credit
		var successCount atomic.Int32
		var wg sync.WaitGroup
		startSignal := make(chan struct{})

		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-startSignal

				tx, txErr := transactor.BeginTx(ctx, nil)
				if txErr != nil {
					return
				}
				defer tx.Rollback()

				deductErr := svc.DeductCreditForTeamStartWithTx(ctx, tx, user.ID, gofakeit.UUID(), gofakeit.UUID())
				if deductErr == nil {
					commitErr := tx.Commit()
					if commitErr == nil {
						successCount.Add(1)
					}
				}
			}()
		}

		close(startSignal)
		wg.Wait()

		// Verify exactly one operation succeeded
		assert.Equal(
			t,
			int32(1),
			successCount.Load(),
			"Attempt %d: Exactly one operation should succeed with 1 credit",
			attempt,
		)

		finalFree, finalPaid, err := svc.GetCreditBalance(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, finalFree+finalPaid, "Attempt %d: Final credits should be 0", attempt)
	}
}
