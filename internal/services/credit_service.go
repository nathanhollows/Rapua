package services

import (
	"context"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/repositories"
)

type CreditRepository interface {
	// DeductCredits deducts a specified number of credits from a user's account.
	DeductCredits(ctx context.Context, userID string, credits int) error
}

type CreditService struct {
	transactor db.Transactor
	creditRepo CreditRepository
	userRepo   repositories.UserRepository
}

func NewCreditService(
	transactor db.Transactor,
	creditRepo CreditRepository,
	userRepo repositories.UserRepository,
) *CreditService {
	return &CreditService{
		transactor: transactor,
		creditRepo: creditRepo,
		userRepo:   userRepo,
	}
}

// GetCreditBalance retrieves the credit balance for a user.
func (s *CreditService) GetCreditBalance(ctx context.Context, userID string) (int, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return user.FreeCredits + user.PaidCredits, nil
}

// AddCredits adds credits to a user's account with a reason for the addition.
func (s *CreditService) AddCredits(ctx context.Context, userID string, credits int, reason string) error {
	// Implement the logic to add credits to the user's account.
	// This may involve updating the credit balance and logging the addition.
	return nil
}

// DeductCredits checks if a user has enough credits and deducts them if they do.
func (s *CreditService) DeductCredits(ctx context.Context, userID string, credits int) (bool, error) {
	balance, err := s.GetCreditBalance(ctx, userID)
	if err != nil {
		return false, err
	}

	if balance < credits {
		return false, nil // Not enough credits
	}

	// Deduct the credits from the user's account.
	err = s.creditRepo.DeductCredits(ctx, userID, credits)
	if err != nil {
		return false, err
	}

	return true, nil // Credits successfully deducted
}
