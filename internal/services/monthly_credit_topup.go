package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/nathanhollows/Rapua/v6/config"
	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

type CreditTopupRepository interface {
	// BulkUpdateCredits updates user credit balances
	BulkUpdateCredits(ctx context.Context, tx *bun.Tx, has int, needs int) error
	// BulkUpdateCreditUpdateNotices updates credit update notices and creates adjustment logs
	BulkUpdateCreditUpdateNotices(
		ctx context.Context,
		tx *bun.Tx,
		has int,
		needs int,
		reason string,
	) error
	// GetMostRecentCreditAdjustmentByReasonPrefix returns the most recent credit adjustment with reason starting with prefix
	GetMostRecentCreditAdjustmentByReasonPrefix(ctx context.Context, reasonPrefix string) (*time.Time, error)
}

type MonthlyCreditTopupService struct {
	transactor db.Transactor
	creditRepo CreditTopupRepository
}

func NewMonthlyCreditTopupService(
	transactor db.Transactor,
	creditRepo CreditTopupRepository,
) *MonthlyCreditTopupService {
	return &MonthlyCreditTopupService{
		transactor: transactor,
		creditRepo: creditRepo,
	}
}

func (s *MonthlyCreditTopupService) TopUpCredits(ctx context.Context) error {
	// Check if monthly top-up already happened this month
	alreadyProcessed, err := s.hasTopUpAlreadyHappenedThisMonth(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if top-up already happened: %w", err)
	}

	if alreadyProcessed {
		// Top-up already processed this month, skip
		return nil
	}

	// Process top-ups for each distinct monthly credit limit
	// This replaces the old regular/educator distinction with flexible per-user limits
	limits := []int{
		config.RegularUserFreeCredits(),
		config.EducatorFreeCredits(),
	}

	// Add any custom domain credit limits
	customDomains := config.CustomDomains()
	for _, limit := range customDomains {
		// Avoid duplicates
		isDuplicate := false
		for _, existing := range limits {
			if existing == limit {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			limits = append(limits, limit)
		}
	}

	// Process users for each credit limit
	for _, creditLimit := range limits {
		if err = s.processUserCredits(ctx, creditLimit, false); err != nil {
			return err
		}
	}

	return nil
}

func (s *MonthlyCreditTopupService) hasTopUpAlreadyHappenedThisMonth(ctx context.Context) (bool, error) {
	// Check for recent monthly top-ups (both regular user and educator use same prefix)
	lastTopUp, err := s.creditRepo.GetMostRecentCreditAdjustmentByReasonPrefix(
		ctx,
		models.CreditAdjustmentReasonPrefixMonthlyTopup,
	)
	if err != nil {
		return false, err
	}

	// If no top-up found, it hasn't happened this month
	if lastTopUp == nil {
		return false, nil
	}

	// Check if the most recent top-up happened this month
	now := time.Now()
	currentYear, currentMonth, _ := now.Date()
	lastYear, lastMonth, _ := lastTopUp.Date()

	return lastYear == currentYear && lastMonth == currentMonth, nil
}

const (
	MaxRetries = 3
	RetryDelay = time.Second * 2
)

func (s *MonthlyCreditTopupService) processUserCredits(ctx context.Context, creditLimit int, _ bool) error {
	for currentCredits := range creditLimit {
		reason := fmt.Sprintf("%s: topped up to %d", models.CreditAdjustmentReasonPrefixMonthlyTopup, creditLimit)

		// Retry logic for this credit level
		err := s.processUserCreditsWithRetry(
			ctx,
			currentCredits,
			creditLimit,
			reason,
			fmt.Sprintf("limit=%d", creditLimit),
		)
		if err != nil {
			// Log the failure but continue with next credit level to avoid partial failures
			slog.Error(
				"processing user credits with retry",
				"credit_limit",
				creditLimit,
				"current_credits",
				currentCredits,
				"error",
				err,
			)

			// For now, we continue processing other credit levels even if one fails
			// In a production system, you might want to implement more sophisticated error handling
			continue
		}
	}

	return nil
}

func (s *MonthlyCreditTopupService) processUserCreditsWithRetry(
	ctx context.Context,
	currentCredits, creditLimit int,
	reason, userType string,
) error {
	var lastErr error

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		err := s.processUserCreditsAtLevel(ctx, currentCredits, creditLimit, reason)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Log the retry attempt
		slog.Error(
			"retrying credit top-up",
			"attempt",
			attempt,
			"user_type",
			userType,
			"current_credits",
			currentCredits,
			"error",
			err,
		)

		// Don't wait after the last attempt
		if attempt < MaxRetries {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(RetryDelay * time.Duration(attempt)): // Exponential backoff
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", MaxRetries, lastErr)
}

func (s *MonthlyCreditTopupService) processUserCreditsAtLevel(
	ctx context.Context,
	currentCredits, creditLimit int,
	reason string,
) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Bulk update credit update notices and create adjustment logs
	err = s.creditRepo.BulkUpdateCreditUpdateNotices(ctx, tx, currentCredits, creditLimit, reason)
	if err != nil {
		return fmt.Errorf("failed to update credit notices: %w", err)
	}

	// Bulk update credits for all users with currentCredits, topping up to creditLimit
	err = s.creditRepo.BulkUpdateCredits(ctx, tx, currentCredits, creditLimit)
	if err != nil {
		return fmt.Errorf("failed to bulk update credits: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
