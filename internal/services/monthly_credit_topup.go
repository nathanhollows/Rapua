package services

import (
	"context"
	"database/sql"
	"fmt"
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

func (s *MonthlyCreditTopupService) processUserCredits(ctx context.Context, creditLimit int, isEducator bool) error {
	var userType string
	if isEducator {
		userType = "educator"
	} else {
		userType = "regular user"
	}

	for currentCredits := 0; currentCredits < creditLimit; currentCredits++ {
		tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return err
		}
		defer tx.Rollback()

		creditsToAdd := creditLimit - currentCredits
		reason := fmt.Sprintf("Monthly free credit top-up for %s: %d credits added", userType, creditsToAdd)

		// Bulk update credit update notices and create adjustment logs
		err = s.creditRepo.BulkUpdateCreditUpdateNotices(ctx, tx, currentCredits, creditLimit, isEducator, reason)
		if err != nil {
			return err
		}

		// Bulk update credits for all users with currentCredits, topping up to creditLimit
		err = s.creditRepo.BulkUpdateCredits(ctx, tx, currentCredits, creditLimit, isEducator)
		if err != nil {
			return err
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
