package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type m20250606103824_User struct {
	bun.BaseModel `bun:"table:users"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID               string         `bun:"id,unique,pk,type:varchar(36)"`
	Name             string         `bun:"name,type:varchar(255)"`
	DisplayName      sql.NullString `bun:"display_name,type:varchar(255),nullzero"`
	Email            string         `bun:"email,unique,pk"`
	EmailVerified    bool           `bun:"email_verified,type:boolean"`
	EmailToken       string         `bun:"email_token,type:varchar(36)"`
	EmailTokenExpiry sql.NullTime   `bun:"email_token_expiry,nullzero"`
	Password         string         `bun:"password,type:varchar(255)"`
	Provider         string         `bun:"provider,type:varchar(255)"`
	Theme            string         `bun:"theme,type:varchar(50),notnull,default:'system'"`
	ShareEmail       bool           `bun:"share_email,type:boolean,notnull,default:false"`
	WorkType         sql.NullString `bun:"work_type,type:varchar(100),nullzero"`

	Instances         []m20241209083639_Instance `bun:"rel:has-many,join:id=user_id"`
	CurrentInstanceID string                     `bun:"current_instance_id,type:varchar(36)"`
	CurrentInstance   m20241209083639_Instance   `bun:"rel:has-one,join:current_instance_id=id"`
}

func init() {
	// Add user preferences columns
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Add display_name column
		_, err := db.NewAddColumn().Model((*m20241209083639_User)(nil)).ColumnExpr("display_name varchar(255)").Exec(ctx)
		if err != nil {
			return fmt.Errorf("add column display_name: %w", err)
		}

		// Add theme column
		_, err = db.NewAddColumn().Model((*m20241209083639_User)(nil)).ColumnExpr("theme varchar(50) NOT NULL DEFAULT 'system'").Exec(ctx)
		if err != nil {
			return fmt.Errorf("add column theme: %w", err)
		}

		// Add share_email column
		_, err = db.NewAddColumn().Model((*m20241209083639_User)(nil)).ColumnExpr("share_email boolean NOT NULL DEFAULT false").Exec(ctx)
		if err != nil {
			return fmt.Errorf("add column share_email: %w", err)
		}
		
		// Add work_type column
		_, err = db.NewAddColumn().Model((*m20241209083639_User)(nil)).ColumnExpr("work_type varchar(100)").Exec(ctx)
		if err != nil {
			return fmt.Errorf("add column work_type: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration.
		columns := []string{"display_name", "theme", "share_email", "work_type"}

		for _, column := range columns {
			_, err := db.NewDropColumn().Model((*m20250606103824_User)(nil)).Column(column).Exec(ctx)
			if err != nil {
				return fmt.Errorf("20250606103824_preferences.go: drop column %s: %w", column, err)
			}
		}

		return nil
	})
}
