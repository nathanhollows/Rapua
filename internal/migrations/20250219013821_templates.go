package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type m20250219013821_Instance struct {
	bun.BaseModel `bun:"table:instances"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID                    string                     `bun:"id,pk,type:varchar(36)"`
	Name                  string                     `bun:"name,type:varchar(255)"`
	UserID                string                     `bun:"user_id,type:varchar(36)"`
	IsTemplate            bool                       `bun:"is_template,type:bool"`
	TemplateID            string                     `bun:"template_id,type:varchar(36),nullzero"`
	StartTime             bun.NullTime               `bun:"start_time,nullzero"`
	EndTime               bun.NullTime               `bun:"end_time,nullzero"`
	Status                m20241209083639_GameStatus `bun:"-"`
	IsQuickStartDismissed bool                       `bun:"is_quick_start_dismissed,type:bool"`

	Teams     []m20241209090041_Team           `bun:"rel:has-many,join:id=instance_id"`
	Locations []m20241209083639_Location       `bun:"rel:has-many,join:id=instance_id"`
	Settings  m20241209083639_InstanceSettings `bun:"rel:has-one,join:id=instance_id"`
}

func init() {
	// Adds the IsTemplate and TemplateID fields to the Instance struct.
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewAddColumn().Model((*m20250219013821_Instance)(nil)).ColumnExpr("is_template bool").Exec(ctx)
		if err != nil {
			return fmt.Errorf("20250219013821_templates.go: add column is_template: %w", err)
		}

		_, err = db.NewAddColumn().
			Model((*m20250219013821_Instance)(nil)).
			ColumnExpr("template_id varchar(36)").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("20250219013821_templates.go: add column template_id: %w", err)
		}

		_, err = db.NewUpdate().Model((*m20250219013821_Instance)(nil)).
			Set("is_template = ?", false).
			Where("is_template IS NULL").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("20250219013821_templates.go: update is_template: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration.
		_, err := db.NewDropColumn().Model((*m20250219013821_Instance)(nil)).Column("is_template").Exec(ctx)
		if err != nil {
			return fmt.Errorf("20250219013821_templates.go: drop column is_template: %w", err)
		}

		_, err = db.NewDropColumn().Model((*m20250219013821_Instance)(nil)).Column("template_id").Exec(ctx)
		if err != nil {
			return fmt.Errorf("20250219013821_templates.go: drop column template_id: %w", err)
		}

		return nil
	})
}
