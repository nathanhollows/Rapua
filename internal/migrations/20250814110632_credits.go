package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// credits + iseducator for users

type m20250814110632_CreditAdjustments struct {
	ID        string    `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UserID    string    `bun:"user_id,notnull,type:varchar(36)"`
	Credits   int       `bun:"credits,type:int,notnull"`
	Reason    string    `bun:"reason,type:varchar(255),notnull"`
}

type m20250814110632_TeamStartLog struct {
	ID         string    `bun:"id,unique,pk,type:varchar(36)"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UserID     string    `bun:"user_id,notnull,type:varchar(36)"`
	InstanceID string    `bun:"instance_id,notnull,type:varchar(36)"`
	TeamID     string    `bun:"team_id,notnull,type:varchar(36)"`
}

type m20250814110632_User struct {
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
	FreeCredits int  `bun:"credits,type:int,default:10"`     // Credits for team starts
	PaidCredits int  `bun:"paid_credits,type:int,default:0"` // Purchased credits
	IsEducator  bool `bun:"is_educator,type:boolean,default:false"`

	Instances         []m20241209083639_Instance          `bun:"rel:has-many,join:id=user_id"`
	CurrentInstanceID string                              `bun:"current_instance_id,type:varchar(36)"`
	CurrentInstance   m20241209083639_Instance            `bun:"rel:has-one,join:current_instance_id=id"`
	TeamStartLogs     []m20250814110632_TeamStartLog      `bun:"rel:has-many,join:id=user_id"`
	CreditAdjustments []m20250814110632_CreditAdjustments `bun:"rel:has-many,join:id=user_id"`
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Create the CreditAdjustments and TeamStartLog tables.
			_, err := db.NewCreateTable().Model(&m20250814110632_CreditAdjustments{}).IfNotExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("create CreditAdjustments table: %w", err)
			}
			_, err = db.NewCreateTable().Model(&m20250814110632_TeamStartLog{}).IfNotExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("create TeamStartLog table: %w", err)
			}

			// Add the FreeCredits, PaidCredits, and IsEducator fields to the User struct.
			_, err = db.NewAddColumn().Model((*m20250814110632_User)(nil)).ColumnExpr("credits int default 10").Exec(ctx)
			if err != nil {
				return fmt.Errorf("add FreeCredits column: %w", err)
			}
			_, err = db.NewAddColumn().Model((*m20250814110632_User)(nil)).ColumnExpr("paid_credits int default 0").Exec(ctx)
			if err != nil {
				return fmt.Errorf("add PaidCredits column: %w", err)
			}
			_, err = db.NewAddColumn().Model((*m20250814110632_User)(nil)).ColumnExpr("is_educator boolean default false").Exec(ctx)
			if err != nil {
				return fmt.Errorf("add IsEducator column: %w", err)
			}

			// Create indexes for lookups.
			_, err = db.NewCreateIndex().Model((*m20250814110632_CreditAdjustments)(nil)).
				Index("idx_credit_adjustments_user_id").Column("user_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_credit_adjustments_user_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20250814110632_TeamStartLog)(nil)).
				Index("idx_team_start_log_user_id").Column("user_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_team_start_log_user_id: %w", err)
			}
			_, err = db.NewCreateIndex().Model((*m20250814110632_TeamStartLog)(nil)).
				Index("idx_team_start_log_instance_id").Column("instance_id").Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_team_start_log_instance_id: %w", err)
			}

			return nil
		}, func(ctx context.Context, db *bun.DB) error {
			// Down migration: drop the CreditAdjustments and TeamStartLog tables.
			_, err := db.NewDropTable().Model(&m20250814110632_CreditAdjustments{}).IfExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("drop CreditAdjustments table: %w", err)
			}
			_, err = db.NewDropTable().Model(&m20250814110632_TeamStartLog{}).IfExists().Exec(context.Background())
			if err != nil {
				return fmt.Errorf("drop TeamStartLog table: %w", err)
			}

			// Remove the FreeCredits, PaidCredits, and IsEducator fields from the User struct.
			_, err = db.NewDropColumn().Model((*m20250814110632_User)(nil)).Column("credits").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop FreeCredits column: %w", err)
			}
			_, err = db.NewDropColumn().Model((*m20250814110632_User)(nil)).Column("paid_credits").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop PaidCredits column: %w", err)
			}
			_, err = db.NewDropColumn().Model((*m20250814110632_User)(nil)).Column("is_educator").Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop IsEducator column: %w", err)
			}

			return nil
		})
}

// Example of how to add and remove fields in a migration
// func init() {
// 	// Adds the IsTemplate and TemplateID fields to the Instance struct.
// 	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
// 		_, err := db.NewAddColumn().Model((*m20250219013821_Instance)(nil)).ColumnExpr("is_template bool").Exec(ctx)
// 		if err != nil {
// 			return fmt.Errorf("20250219013821_templates.go: add column is_template: %w", err)
// 		}
//
// 		_, err = db.NewAddColumn().Model((*m20250219013821_Instance)(nil)).ColumnExpr("template_id varchar(36)").Exec(ctx)
// 		if err != nil {
// 			return fmt.Errorf("20250219013821_templates.go: add column template_id: %w", err)
// 		}
//
// 		_, err = db.NewUpdate().Model((*m20250219013821_Instance)(nil)).
// 			Set("is_template = ?", false).
// 			Where("is_template IS NULL").
// 			Exec(ctx)
// 		if err != nil {
// 			return fmt.Errorf("20250219013821_templates.go: update is_template: %w", err)
// 		}
//
// 		return nil
// 	}, func(ctx context.Context, db *bun.DB) error {
// 		// Down migration.
// 		_, err := db.NewDropColumn().Model((*m20250219013821_Instance)(nil)).Column("is_template").Exec(ctx)
// 		if err != nil {
// 			return fmt.Errorf("20250219013821_templates.go: drop column is_template: %w", err)
// 		}
//
// 		_, err = db.NewDropColumn().Model((*m20250219013821_Instance)(nil)).Column("template_id").Exec(ctx)
// 		if err != nil {
// 			return fmt.Errorf("20250219013821_templates.go: drop column template_id: %w", err)
// 		}
//
// 		return nil
// 	})
// }
