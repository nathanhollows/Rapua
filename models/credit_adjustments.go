package models

import (
	"database/sql"
	"time"
)

type CreditAdjustments struct {
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`

	ID                string         `bun:"id,unique,pk,type:varchar(36)"`
	UserID            string         `bun:"user_id,notnull,type:varchar(36)"`
	Credits           int            `bun:"credits,type:int,notnull"`
	Reason            string         `bun:"reason,type:varchar(255),notnull"`
	CreditPurchaseID  sql.NullString `bun:"credit_purchase_id,type:varchar(36),nullzero"`

	CreditPurchase *CreditPurchase `bun:"rel:belongs-to,join:credit_purchase_id=id"`
}

// Credit adjustment reason prefixes.
const (
	CreditAdjustmentReasonPrefixMigration = "Migration"
	//nolint:gosec // String is not sensitive
	CreditAdjustmentReasonPrefixMonthlyTopup = "Monthly free credit top-up"
	CreditAdjustmentReasonPrefixPurchase     = "Purchase"
	CreditAdjustmentReasonPrefixAdmin        = "Admin"
	CreditAdjustmentReasonPrefixGift         = "Gift"
)
