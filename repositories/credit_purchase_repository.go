package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun"
)

type CreditPurchaseRepository struct {
	db *bun.DB
}

func NewCreditPurchaseRepository(db *bun.DB) *CreditPurchaseRepository {
	return &CreditPurchaseRepository{
		db: db,
	}
}

// Create creates a new credit purchase record.
func (r *CreditPurchaseRepository) Create(ctx context.Context, purchase *models.CreditPurchase) error {
	if purchase.UserID == "" {
		return errors.New("user_id is required")
	}
	if purchase.Credits <= 0 {
		return errors.New("credits must be greater than zero")
	}
	if purchase.AmountPaid < 0 {
		return errors.New("amount_paid cannot be negative")
	}
	if purchase.StripeSessionID == "" {
		return errors.New("stripe_session_id is required")
	}
	if purchase.Status == "" {
		purchase.Status = models.CreditPurchaseStatusPending
	}

	_, err := r.db.NewInsert().
		Model(purchase).
		Exec(ctx)
	return err
}

// CreateWithTx creates a new credit purchase record within a transaction.
func (r *CreditPurchaseRepository) CreateWithTx(
	ctx context.Context,
	tx *bun.Tx,
	purchase *models.CreditPurchase,
) error {
	if purchase.UserID == "" {
		return errors.New("user_id is required")
	}
	if purchase.Credits <= 0 {
		return errors.New("credits must be greater than zero")
	}
	if purchase.AmountPaid < 0 {
		return errors.New("amount_paid cannot be negative")
	}
	if purchase.StripeSessionID == "" {
		return errors.New("stripe_session_id is required")
	}
	if purchase.Status == "" {
		purchase.Status = models.CreditPurchaseStatusPending
	}

	_, err := tx.NewInsert().
		Model(purchase).
		Exec(ctx)
	return err
}

// GetByStripeSessionID retrieves a credit purchase by Stripe session ID.
func (r *CreditPurchaseRepository) GetByStripeSessionID(
	ctx context.Context,
	sessionID string,
) (*models.CreditPurchase, error) {
	var purchase models.CreditPurchase
	err := r.db.NewSelect().
		Model(&purchase).
		Where("stripe_session_id = ?", sessionID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &purchase, nil
}

// GetByID retrieves a credit purchase by ID.
func (r *CreditPurchaseRepository) GetByID(ctx context.Context, id string) (*models.CreditPurchase, error) {
	var purchase models.CreditPurchase
	err := r.db.NewSelect().
		Model(&purchase).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &purchase, nil
}

// GetByUserID retrieves all credit purchases for a user.
func (r *CreditPurchaseRepository) GetByUserID(ctx context.Context, userID string) ([]models.CreditPurchase, error) {
	var purchases []models.CreditPurchase
	err := r.db.NewSelect().
		Model(&purchases).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return purchases, nil
}

// GetByUserIDWithPagination retrieves credit purchases for a user with pagination.
func (r *CreditPurchaseRepository) GetByUserIDWithPagination(
	ctx context.Context,
	userID string,
	limit, offset int,
) ([]models.CreditPurchase, error) {
	var purchases []models.CreditPurchase
	err := r.db.NewSelect().
		Model(&purchases).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return purchases, nil
}

// UpdateStatus updates the status of a credit purchase.
func (r *CreditPurchaseRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	// Validate status
	validStatuses := map[string]bool{
		models.CreditPurchaseStatusPending:   true,
		models.CreditPurchaseStatusCompleted: true,
		models.CreditPurchaseStatusFailed:    true,
		models.CreditPurchaseStatusCancelled: true,
	}
	if !validStatuses[status] {
		return errors.New("invalid purchase status: " + status)
	}

	_, err := r.db.NewUpdate().
		Model(&models.CreditPurchase{}).
		Set("status = ?", status).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// UpdateStatusWithTx updates the status of a credit purchase within a transaction.
func (r *CreditPurchaseRepository) UpdateStatusWithTx(ctx context.Context, tx *bun.Tx, id string, status string) error {
	// Validate status
	validStatuses := map[string]bool{
		models.CreditPurchaseStatusPending:   true,
		models.CreditPurchaseStatusCompleted: true,
		models.CreditPurchaseStatusFailed:    true,
		models.CreditPurchaseStatusCancelled: true,
	}
	if !validStatuses[status] {
		return errors.New("invalid purchase status: " + status)
	}

	_, err := tx.NewUpdate().
		Model(&models.CreditPurchase{}).
		Set("status = ?", status).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// UpdateStripePaymentID updates the Stripe payment ID for a purchase.
func (r *CreditPurchaseRepository) UpdateStripePaymentIDWithTx(
	ctx context.Context,
	tx *bun.Tx,
	id string,
	paymentID string,
) error {
	_, err := tx.NewUpdate().
		Model(&models.CreditPurchase{}).
		Set("stripe_payment_id = ?", paymentID).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// UpdateReceiptURLWithTx updates the receipt URL for a purchase.
func (r *CreditPurchaseRepository) UpdateReceiptURLWithTx(
	ctx context.Context,
	tx *bun.Tx,
	id string,
	receiptURL string,
) error {
	_, err := tx.NewUpdate().
		Model(&models.CreditPurchase{}).
		Set("receipt_url = ?", receiptURL).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// DeleteByUserID deletes all credit purchases for a user within a transaction.
func (r *CreditPurchaseRepository) DeleteByUserID(ctx context.Context, tx *bun.Tx, userID string) error {
	_, err := tx.NewDelete().
		Model(&models.CreditPurchase{}).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}
