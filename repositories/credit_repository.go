package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
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

func (r *CreditRepository) BulkUpdateCredits(ctx context.Context, tx *bun.Tx, has int, needs int, isEducator bool) error {
	_, err := tx.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = ?", needs).
		Where("free_credits = ? AND is_educator = ?", has, isEducator).
		Exec(ctx)
	return err
}

func (r *CreditRepository) BulkUpdateCreditUpdateNotices(ctx context.Context, tx *bun.Tx, has int, needs int, isEducator bool, reason string) error {
	// First, get all users that need top-up
	var users []struct {
		ID string `bun:"id"`
	}
	err := tx.NewSelect().
		Model(&models.User{}).
		Column("id").
		Where("free_credits = ? AND is_educator = ?", has, isEducator).
		Scan(ctx, &users)
	if err != nil {
		return err
	}

	// If no users need top-up, return early
	if len(users) == 0 {
		return nil
	}

	// Create credit adjustments for each user
	adjustments := make([]models.CreditAdjustments, len(users))
	for i, user := range users {
		adjustments[i] = models.CreditAdjustments{
			ID:      uuid.NewString(),
			UserID:  user.ID,
			Credits: needs - has, // Amount being added
			Reason:  reason,
		}
	}

	// Bulk insert credit adjustments
	_, err = tx.NewInsert().
		Model(&adjustments).
		Exec(ctx)
	
	return err
}

// GetMostRecentCreditAdjustmentByReasonPrefix returns the most recent credit adjustment with reason starting with prefix
func (r *CreditRepository) GetMostRecentCreditAdjustmentByReasonPrefix(ctx context.Context, reasonPrefix string) (*time.Time, error) {
	var adjustment models.CreditAdjustments
	err := r.db.NewSelect().
		Model(&adjustment).
		Column("created_at").
		Where("reason LIKE ?", reasonPrefix+"%").
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No adjustments found, return nil (not an error)
			return nil, nil
		}
		return nil, err
	}
	
	return &adjustment.CreatedAt, nil
}
