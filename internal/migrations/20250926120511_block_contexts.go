package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v5/blocks"
	"github.com/uptrace/bun"
)

type m20250926120511_Block struct {
	bun.BaseModel `bun:"table:blocks"`

	ID                 string              `bun:"id,pk,notnull"`
	LocationID         string              `bun:"location_id,notnull"`
	Type               string              `bun:"type,type:int"`
	Context            blocks.BlockContext `bun:"context,type:string"` // ADDED
	Data               json.RawMessage     `bun:"data,type:jsonb"`
	Ordering           int                 `bun:"ordering,type:int"`
	Points             int                 `bun:"points,type:int"`
	ValidationRequired bool                `bun:"validation_required,type:bool"`
}

type m20250926120511_Location struct {
	bun.BaseModel `bun:"table:locations"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID           string  `bun:"id,pk,notnull"`
	Name         string  `bun:"name,type:varchar(255)"`
	InstanceID   string  `bun:"instance_id,notnull"`
	MarkerID     string  `bun:"marker_id,notnull"`
	ContentID    string  `bun:"content_id,notnull"`
	Criteria     string  `bun:"criteria,type:varchar(255)"`
	Order        int     `bun:"order,type:int"`
	TotalVisits  int     `bun:"total_visits,type:int"`
	CurrentCount int     `bun:"current_count,type:int"`
	AvgDuration  float64 `bun:"avg_duration,type:float"`
	Points       int     `bun:"points,"`

	// Removed
	// Clues     []m20241209083639_Clue   `bun:"rel:has-many,join:id=location_id"`
	Instance m20241209083639_Instance `bun:"rel:has-one,join:instance_id=id"`
	Marker   m20241209083639_Marker   `bun:"rel:has-one,join:marker_id=code"`
	Blocks   []m20250926120511_Block  `bun:"rel:has-many,join:id=location_id"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Add context column to blocks table
		_, err := db.ExecContext(ctx, `
			ALTER TABLE blocks
			ADD COLUMN context VARCHAR(50) DEFAULT ?;
		`,
			blocks.ContextLocationContent,
		)
		if err != nil {
			return fmt.Errorf("add context column to blocks: %w", err)
		}

		// Set all existing blocks to 'content' context
		_, err = db.NewUpdate().
			Model((*m20250926120511_Block)(nil)).
			Set("context = ?", blocks.ContextLocationContent).
			Where("context IS NULL OR context = ''").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("set default context for existing blocks: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Drop context column from blocks table
		_, err := db.ExecContext(ctx, `
			ALTER TABLE blocks
			DROP COLUMN context;
		`)
		if err != nil {
			return fmt.Errorf("drop context column from blocks: %w", err)
		}

		return nil
	})
}
