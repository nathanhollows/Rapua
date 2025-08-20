package repositories

import (
	"context"
	"errors"

	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun"
)

type CreditRepository struct {
	db *bun.DB
}

func NewCreditRepository(db *bun.DB) *CreditRepository {
	return &CreditRepository{
		db: db,
	}
}

func (r *CreditRepository) UpdateCredits(ctx context.Context, userID string, freeCredits int, paidCredits int) error {
	if freeCredits < 0 || paidCredits < 0 {
		return errors.New("credits cannot be negative")
	}
	_, err := r.db.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = ?, paid_credits = ?", freeCredits, paidCredits).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (r *CreditRepository) UpdateCreditsWithTx(ctx context.Context, tx *bun.Tx, userID string, freeCredits int, paidCredits int) error {
	if freeCredits < 0 || paidCredits < 0 {
		return errors.New("credits cannot be negative")
	}
	_, err := tx.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = ?, paid_credits = ?", freeCredits, paidCredits).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

// GetCreditAdjustmentsByUserID returns all credit adjustments for a user
func (r *CreditRepository) GetCreditAdjustmentsByUserID(ctx context.Context, userID string) ([]models.CreditAdjustments, error) {
	var adjustments []models.CreditAdjustments
	err := r.db.NewSelect().
		Model(&adjustments).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return adjustments, nil
}

// GetCreditAdjustmentsByUserIDWithPagination returns credit adjustments for a user with pagination
func (r *CreditRepository) GetCreditAdjustmentsByUserIDWithPagination(ctx context.Context, userID string, limit, offset int) ([]models.CreditAdjustments, error) {
	var adjustments []models.CreditAdjustments
	err := r.db.NewSelect().
		Model(&adjustments).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return adjustments, nil
}

// SaveCreditAdjustment saves a new credit adjustment record
func (r *CreditRepository) CreateCreditAdjustmentWithTx(ctx context.Context, tx *bun.Tx, adjustment *models.CreditAdjustments) error {
	if adjustment.UserID == "" {
		return errors.New("userID is required for credit adjustment")
	}
	if adjustment.Credits == 0 {
		return errors.New("credits cannot be zero")
	}
	if adjustment.Reason == "" {
		return errors.New("reason is required for credit adjustment")
	}
	_, err := tx.NewInsert().
		Model(adjustment).
		Exec(ctx)
	return err
}
