package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/uptrace/bun"
)

type CreditTopupRepository interface {
	// BulkUpdateCredits updates user credit balances
	BulkUpdateCredits(ctx context.Context, tx *bun.Tx, has int, needs int, isEducator bool) error
	// BulkUpdateCreditUpdateNotices updates credit update notices and creates adjustment logs
	BulkUpdateCreditUpdateNotices(ctx context.Context, tx *bun.Tx, has int, needs int, isEducator bool, reason string) error
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

const (
	RegularUserFreeCredits = 10
	EducatorFreeCredits    = 50
)

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

	// Process regular users (up to 10 credits)
	if err := s.processUserCredits(ctx, RegularUserFreeCredits, false); err != nil {
		return err
	}

	// Process educators (up to 50 credits)
	if err := s.processUserCredits(ctx, EducatorFreeCredits, true); err != nil {
		return err
	}

	return nil
}

func (s *MonthlyCreditTopupService) hasTopUpAlreadyHappenedThisMonth(ctx context.Context) (bool, error) {
	// Check for recent regular user top-ups
	regularUserPrefix := "Monthly free credit top-up for regular user"
	lastRegularTopUp, err := s.creditRepo.GetMostRecentCreditAdjustmentByReasonPrefix(ctx, regularUserPrefix)
	if err != nil {
		return false, err
	}

	// Check for recent educator top-ups
	educatorPrefix := "Monthly free credit top-up for educator"
	lastEducatorTopUp, err := s.creditRepo.GetMostRecentCreditAdjustmentByReasonPrefix(ctx, educatorPrefix)
	if err != nil {
		return false, err
	}

	now := time.Now()
	currentYear, currentMonth, _ := now.Date()

	// Check if either regular user or educator top-up happened this month
	if lastRegularTopUp != nil {
		lastYear, lastMonth, _ := lastRegularTopUp.Date()
		if lastYear == currentYear && lastMonth == currentMonth {
			return true, nil
		}
	}

	if lastEducatorTopUp != nil {
		lastYear, lastMonth, _ := lastEducatorTopUp.Date()
		if lastYear == currentYear && lastMonth == currentMonth {
			return true, nil
		}
	}

	return false, nil
}

const (
	MaxRetries = 3
	RetryDelay = time.Second * 2
)

func (s *MonthlyCreditTopupService) processUserCredits(ctx context.Context, creditLimit int, isEducator bool) error {
	var userType string
	if isEducator {
		userType = "educator"
	} else {
		userType = "regular user"
	}

	for currentCredits := 0; currentCredits < creditLimit; currentCredits++ {
		creditsToAdd := creditLimit - currentCredits
		reason := fmt.Sprintf("Monthly free credit top-up for %s: %d credits added", userType, creditsToAdd)

		// Retry logic for this credit level
		err := s.processUserCreditsWithRetry(ctx, currentCredits, creditLimit, isEducator, reason, userType)
		if err != nil {
			// Log the failure but continue with next credit level to avoid partial failures
			log.Printf("Failed to process credits for %s users with %d credits after %d retries: %v",
				userType, currentCredits, MaxRetries, err)

			// For now, we continue processing other credit levels even if one fails
			// In a production system, you might want to implement more sophisticated error handling
			continue
		}
	}

	return nil
}

func (s *MonthlyCreditTopupService) processUserCreditsWithRetry(ctx context.Context, currentCredits, creditLimit int, isEducator bool, reason, userType string) error {
	var lastErr error

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		err := s.processUserCreditsAtLevel(ctx, currentCredits, creditLimit, isEducator, reason)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Log the retry attempt
		log.Printf("Attempt %d/%d failed for %s users with %d credits: %v",
			attempt, MaxRetries, userType, currentCredits, err)

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

func (s *MonthlyCreditTopupService) processUserCreditsAtLevel(ctx context.Context, currentCredits, creditLimit int, isEducator bool, reason string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Bulk update credit update notices and create adjustment logs
	err = s.creditRepo.BulkUpdateCreditUpdateNotices(ctx, tx, currentCredits, creditLimit, isEducator, reason)
	if err != nil {
		return fmt.Errorf("failed to update credit notices: %w", err)
	}

	// Bulk update credits for all users with currentCredits, topping up to creditLimit
	err = s.creditRepo.BulkUpdateCredits(ctx, tx, currentCredits, creditLimit, isEducator)
	if err != nil {
		return fmt.Errorf("failed to bulk update credits: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
