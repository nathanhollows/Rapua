package models

import (
	"database/sql"
	"time"

	"github.com/nathanhollows/Rapua/v5/config"
)

// CreditPurchase represents a record of credit purchases via Stripe.
type CreditPurchase struct {
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID               string         `bun:"id,unique,pk,type:varchar(36)"`
	UserID           string         `bun:"user_id,notnull,type:varchar(36)"`
	Credits          int            `bun:"credits,type:int,notnull"`
	AmountPaid       int            `bun:"amount_paid,type:int,notnull"` // Amount in cents
	StripePaymentID  sql.NullString `bun:"stripe_payment_id,type:varchar(255),nullzero"`
	StripeSessionID  string         `bun:"stripe_session_id,type:varchar(255),notnull,unique"`
	StripeCustomerID sql.NullString `bun:"stripe_customer_id,type:varchar(255),nullzero"`
	ReceiptURL       sql.NullString `bun:"receipt_url,type:varchar(500),nullzero"`
	Status           string         `bun:"status,type:varchar(20),notnull,default:'pending'"`

	User User `bun:"rel:belongs-to,join:user_id=id"`
}

// Credit purchase status constants.
const (
	CreditPurchaseStatusPending   = "pending"
	CreditPurchaseStatusCompleted = "completed"
	CreditPurchaseStatusFailed    = "failed"
	CreditPurchaseStatusCancelled = "cancelled"
)

// CalculatePurchaseAmount calculates the total amount in cents for a credit purchase.
func CalculatePurchaseAmount(credits int) int {
	return credits * config.CreditPriceCents()
}
