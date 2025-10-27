package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v5/config"
	"github.com/nathanhollows/Rapua/v5/helpers"
	"github.com/uptrace/bun"
)

type m20251017213848_CreditAdjustments struct {
	bun.BaseModel    `bun:"table:credit_adjustments"`
	ID               string         `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt        time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UserID           string         `bun:"user_id,notnull,type:varchar(36)"`
	Credits          int            `bun:"credits,type:int,notnull"`
	Reason           string         `bun:"reason,type:varchar(255),notnull"`
	CreditPurchaseID sql.NullString `bun:"credit_purchase_id,type:varchar(36),nullzero"`
}

type m20251017213848_TeamStartLog struct {
	bun.BaseModel `bun:"table:team_start_logs"`
	ID            string    `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UserID        string    `bun:"user_id,notnull,type:varchar(36)"`
	InstanceID    string    `bun:"instance_id,notnull,type:varchar(36)"`
	TeamID        string    `bun:"team_id,notnull,type:varchar(36)"`
}

type m20251017213848_CreditPurchase struct {
	bun.BaseModel    `bun:"table:credit_purchases"`
	ID               string         `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt        time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	UserID           string         `bun:"user_id,notnull,type:varchar(36)"`
	Credits          int            `bun:"credits,type:int,notnull"`
	AmountPaid       int            `bun:"amount_paid,type:int,notnull"`
	StripePaymentID  sql.NullString `bun:"stripe_payment_id,type:varchar(255),nullzero"`
	StripeSessionID  string         `bun:"stripe_session_id,type:varchar(255),notnull,unique"`
	StripeCustomerID sql.NullString `bun:"stripe_customer_id,type:varchar(255),nullzero"`
	ReceiptURL       sql.NullString `bun:"receipt_url,type:varchar(500),nullzero"`
	Status           string         `bun:"status,type:varchar(20),notnull,default:'pending'"`
}

type m20251017213848_User struct {
	bun.BaseModel `bun:"table:users"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID               string       `bun:"id,unique,pk,type:varchar(36)"`
	Name             string       `bun:"name,type:varchar(255)"`
	Email            string       `bun:"email,unique,pk"`
	EmailVerified    bool         `bun:"email_verified,type:boolean"`
	EmailToken       string       `bun:"email_token,type:varchar(36)"`
	EmailTokenExpiry sql.NullTime `bun:"email_token_expiry,nullzero"`
	Password         string       `bun:"password,type:varchar(255)"`
	Provider         string       `bun:"provider,type:varchar(255)"`
	// New fields:
	FreeCredits        int            `bun:"free_credits,type:int,default:10"`         // Credits for team starts
	PaidCredits        int            `bun:"paid_credits,type:int,default:0"`          // Purchased credits
	MonthlyCreditLimit int            `bun:"monthly_credit_limit,type:int,default:10"` // Monthly credit allocation
	StripeCustomerID   sql.NullString `bun:"stripe_customer_id,type:varchar(255),nullzero"`

	Instances         []m20241209083639_Instance          `bun:"rel:has-many,join:id=user_id"`
	CurrentInstanceID string                              `bun:"current_instance_id,type:varchar(36)"`
	CurrentInstance   m20241209083639_Instance            `bun:"rel:has-one,join:current_instance_id=id"`
	TeamStartLogs     []m20251017213848_TeamStartLog      `bun:"rel:has-many,join:id=user_id"`
	CreditAdjustments []m20251017213848_CreditAdjustments `bun:"rel:has-many,join:id=user_id"`
	CreditPurchases   []m20251017213848_CreditPurchase    `bun:"rel:has-many,join:id=user_id"`
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Create the CreditAdjustments and TeamStartLog tables.
			_, err := db.NewCreateTable().
				Model(&m20251017213848_CreditAdjustments{}).
				IfNotExists().
				Exec(context.Background())
			if err != nil {
				return fmt.Errorf("create CreditAdjustments table: %w", err)
			}
			_, err = db.NewCreateTable().Model(&m20251017213848_TeamStartLog{}).IfNotExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("create TeamStartLog table: %w", err)
			}

			// Create the CreditPurchases table for Stripe integration.
			_, err = db.NewCreateTable().
				Model(&m20251017213848_CreditPurchase{}).
				IfNotExists().
				Exec(context.Background())
			if err != nil {
				return fmt.Errorf("create CreditPurchases table: %w", err)
			}

			// Add the FreeCredits, PaidCredits, and MonthlyCreditLimit fields to the User struct.
			// Ignore duplicate column errors if columns already exist
			_, err = db.NewAddColumn().
				Model((*m20251017213848_User)(nil)).
				ColumnExpr("free_credits int default 10").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("add FreeCredits column: %w", err)
			}
			_, err = db.NewAddColumn().
				Model((*m20251017213848_User)(nil)).
				ColumnExpr("paid_credits int default 0").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("add PaidCredits column: %w", err)
			}
			_, err = db.NewAddColumn().
				Model((*m20251017213848_User)(nil)).
				ColumnExpr("monthly_credit_limit int default 10").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("add MonthlyCreditLimit column: %w", err)
			}
			_, err = db.NewAddColumn().
				Model((*m20251017213848_User)(nil)).
				ColumnExpr("stripe_customer_id varchar(255)").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("add StripeCustomerID column: %w", err)
			}

			// Create indexes for lookups.
			_, err = db.NewCreateIndex().Model((*m20251017213848_CreditAdjustments)(nil)).
				Index("idx_credit_adjustments_user_id").Column("user_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_credit_adjustments_user_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20251017213848_CreditAdjustments)(nil)).
				Index("idx_credit_adjustments_purchase_id").Column("credit_purchase_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_credit_adjustments_purchase_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20251017213848_TeamStartLog)(nil)).
				Index("idx_team_start_log_user_id").Column("user_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_team_start_log_user_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20251017213848_TeamStartLog)(nil)).
				Index("idx_team_start_log_instance_id").Column("instance_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_team_start_log_instance_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20251017213848_CreditPurchase)(nil)).
				Index("idx_credit_purchases_user_id").Column("user_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_credit_purchases_user_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20251017213848_CreditPurchase)(nil)).
				Index("idx_credit_purchases_stripe_session_id").Column("stripe_session_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_credit_purchases_stripe_session_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20251017213848_CreditPurchase)(nil)).
				Index("idx_credit_purchases_status").Column("status").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_credit_purchases_status: %w", err)
			}

			// Fetch all existing users and set their monthly_credit_limit based on their email
			var existingUsersForLimitUpdate []m20251017213848_User
			err = db.NewSelect().Model(&existingUsersForLimitUpdate).Column("id", "email").Scan(ctx)
			if err != nil {
				return fmt.Errorf("get existing users for credit limit update: %w", err)
			}

			// Update each user's monthly_credit_limit based on their email
			for _, user := range existingUsersForLimitUpdate {
				// Determine the appropriate credit limit for this user's email
				creditLimit := config.GetFreeCreditsForEmail(user.Email, helpers.IsEducationalEmailHeuristic)

				// Update this user's monthly_credit_limit
				_, err = db.NewUpdate().Model((*m20251017213848_User)(nil)).
					Set("monthly_credit_limit = ?", creditLimit).
					Where("id = ?", user.ID).
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("set monthly credit limit for user %s: %w", user.ID, err)
				}
			}

			// Set all users' free_credits to their monthly_credit_limit
			_, err = db.NewUpdate().Model((*m20251017213848_User)(nil)).
				Set("free_credits = monthly_credit_limit").
				Where("1 = 1"). // Match all users
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("set free credits to monthly limit: %w", err)
			}

			// Add 500 paid credits to all existing users as a thank you gift
			_, err = db.NewUpdate().Model((*m20251017213848_User)(nil)).
				Set("paid_credits = paid_credits + 500").
				Where("paid_credits < 500"). // Only update users with less than 500 paid credits
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("add 500 paid credits to existing users: %w", err)
			}

			// Get all existing users to create adjustment records
			var existingUsers []m20251017213848_User
			err = db.NewSelect().Model(&existingUsers).Column("id").Scan(ctx)
			if err != nil {
				return fmt.Errorf("get existing users for adjustment records: %w", err)
			}

			// Create adjustment records for each user
			adjustments := make([]m20251017213848_CreditAdjustments, len(existingUsers))
			for i, user := range existingUsers {
				adjustments[i] = m20251017213848_CreditAdjustments{
					ID:        uuid.New().String(),
					CreatedAt: time.Now(),
					UserID:    user.ID,
					//nolint:mnd // 500 is the thank you gift amount
					Credits: 500,
					Reason:  "Gift: Founders pack - thank you for being an early user!",
				}
			}

			if len(adjustments) > 0 {
				_, err = db.NewInsert().Model(&adjustments).Exec(ctx)
				if err != nil {
					return fmt.Errorf("create adjustment records for existing users: %w", err)
				}
			}

			return nil
		}, func(ctx context.Context, db *bun.DB) error {
			// Down migration: drop the CreditAdjustments, TeamStartLog, and CreditPurchases tables.
			_, err := db.NewDropTable().Model(&m20251017213848_CreditPurchase{}).IfExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("drop CreditPurchases table: %w", err)
			}
			_, err = db.NewDropTable().Model(&m20251017213848_CreditAdjustments{}).IfExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("drop CreditAdjustments table: %w", err)
			}
			_, err = db.NewDropTable().Model(&m20251017213848_TeamStartLog{}).IfExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("drop TeamStartLog table: %w", err)
			}

			// Remove the FreeCredits, PaidCredits, MonthlyCreditLimit, and StripeCustomerID fields from the User struct.
			_, err = db.NewDropColumn().Model((*m20251017213848_User)(nil)).Column("stripe_customer_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop StripeCustomerID column: %w", err)
			}
			_, err = db.NewDropColumn().Model((*m20251017213848_User)(nil)).Column("free_credits").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop FreeCredits column: %w", err)
			}
			_, err = db.NewDropColumn().Model((*m20251017213848_User)(nil)).Column("paid_credits").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop PaidCredits column: %w", err)
			}
			_, err = db.NewDropColumn().Model((*m20251017213848_User)(nil)).Column("monthly_credit_limit").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop MonthlyCreditLimit column: %w", err)
			}

			return nil
		})
}
