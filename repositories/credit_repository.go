package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/models"
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

// AddCreditsWithTx atomically increments credits without a read-modify-write cycle.
// This prevents lost updates from concurrent operations.
func (r *CreditRepository) AddCreditsWithTx(
	ctx context.Context,
	tx *bun.Tx,
	userID string,
	freeCreditsToAdd int,
	paidCreditsToAdd int,
) error {
	result, err := tx.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = free_credits + ?", freeCreditsToAdd).
		Set("paid_credits = paid_credits + ?", paidCreditsToAdd).
		Where("id = ?", userID).
		Exec(ctx)
	if err != nil {
		return err
	}

	// Check if any rows were affected (user must exist)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

// GetCreditAdjustmentsByUserID returns all credit adjustments for a user.
func (r *CreditRepository) GetCreditAdjustmentsByUserID(
	ctx context.Context,
	userID string,
) ([]models.CreditAdjustments, error) {
	var adjustments []models.CreditAdjustments
	err := r.db.NewSelect().
		Model(&adjustments).
		Relation("CreditPurchase").
		Where("credit_adjustments.user_id = ?", userID).
		Order("credit_adjustments.created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return adjustments, nil
}

// GetCreditAdjustmentsByUserIDWithPagination returns credit adjustments for a user with pagination.
func (r *CreditRepository) GetCreditAdjustmentsByUserIDWithPagination(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]models.CreditAdjustments, error) {
	var adjustments []models.CreditAdjustments
	err := r.db.NewSelect().
		Model(&adjustments).
		Relation("CreditPurchase").
		Where("credit_adjustments.user_id = ?", userID).
		Order("credit_adjustments.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return adjustments, nil
}

// CreateCreditAdjustmentWithTx saves a new credit adjustment record.
func (r *CreditRepository) CreateCreditAdjustmentWithTx(
	ctx context.Context,
	tx *bun.Tx,
	adjustment *models.CreditAdjustments,
) error {
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

func (r *CreditRepository) BulkUpdateCredits(
	ctx context.Context,
	tx *bun.Tx,
	has int,
	needs int,
) error {
	// Update users to their monthly credit limit
	_, err := tx.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = monthly_credit_limit").
		Where("free_credits = ?", has).
		Where("monthly_credit_limit = ?", needs).
		Exec(ctx)
	return err
}

func (r *CreditRepository) BulkUpdateCreditUpdateNotices(
	ctx context.Context,
	tx *bun.Tx,
	has int,
	needs int,
	reason string,
) error {
	// First, get all users that need top-up
	var users []struct {
		ID string `bun:"id"`
	}
	err := tx.NewSelect().
		Model(&models.User{}).
		Column("id").
		Where("free_credits = ?", has).
		Where("monthly_credit_limit = ?", needs).
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

// GetMostRecentCreditAdjustmentByReasonPrefix returns the most recent credit adjustment with reason starting with prefix.
func (r *CreditRepository) GetMostRecentCreditAdjustmentByReasonPrefix(
	ctx context.Context,
	reasonPrefix string,
) (*time.Time, error) {
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
			//nolint:nilnil // Returning nil, nil is intentional here - nil time indicates no record found, nil error indicates no error occurred
			return nil, nil
		}
		return nil, err
	}

	return &adjustment.CreatedAt, nil
}

// DeductOneCreditWithTx atomically deducts one credit, preferring free credits over paid credits.
// Returns an error if the user doesn't have sufficient credits.
func (r *CreditRepository) DeductOneCreditWithTx(ctx context.Context, tx *bun.Tx, userID string) error {
	// Use a single atomic UPDATE with conditional logic
	// The CASE statement evaluates free_credits at the start of the UPDATE
	result, err := tx.NewUpdate().
		Model(&models.User{}).
		Set("free_credits = CASE WHEN free_credits > 0 THEN free_credits - 1 ELSE free_credits END").
		Set("paid_credits = CASE WHEN free_credits = 0 AND paid_credits > 0 THEN paid_credits - 1 ELSE paid_credits END").
		Where("id = ? AND (free_credits > 0 OR paid_credits > 0)", userID).
		Exec(ctx)

	if err != nil {
		return err
	}

	// Check if any rows were affected (meaning user had sufficient credits)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		// No rows were updated, meaning user doesn't have sufficient credits
		return errors.New("insufficient credits to start team")
	}

	return nil
}

// DeleteCreditAdjustmentsByUserID deletes all credit adjustments for a user within a transaction.
func (r *CreditRepository) DeleteCreditAdjustmentsByUserID(ctx context.Context, tx *bun.Tx, userID string) error {
	_, err := tx.NewDelete().
		Model(&models.CreditAdjustments{}).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}
