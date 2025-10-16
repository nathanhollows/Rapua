package models

import "time"

type CreditAdjustments struct {
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`

	ID      string `bun:"id,unique,pk,type:varchar(36)"`
	UserID  string `bun:"user_id,notnull,type:varchar(36)"`
	Credits int    `bun:"credits,type:int,notnull"`
	Reason  string `bun:"reason,type:varchar(255),notnull"`
}

// Credit adjustment reason prefixes.
const (
	CreditAdjustmentReasonPrefixMigration    = "Migration"
	CreditAdjustmentReasonPrefixMonthlyTopup = "Monthly free credit top-up"
	CreditAdjustmentReasonPrefixPurchase     = "Purchase"
	CreditAdjustmentReasonPrefixAdmin        = "Admin"
	CreditAdjustmentReasonPrefixGift         = "Gift"
)
