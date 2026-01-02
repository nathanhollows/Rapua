package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// m20251230100000_GameState is the migration-specific model for game_state table.
type m20251230100000_GameState struct {
	bun.BaseModel `bun:"table:game_state"`

	InstanceID string         `bun:"instance_id,pk,type:varchar(36)"`
	Scope      string         `bun:"scope,pk,type:varchar(20)"`
	EntityID   string         `bun:"entity_id,pk,type:varchar(36)"`
	Data       map[string]any `bun:"data,type:jsonb,notnull,default:'{}'"`
	Version    int            `bun:"version,type:int,notnull,default:1"`
	UpdatedAt  time.Time      `bun:"updated_at,notnull,default:current_timestamp"`
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Create game_state table
			_, err := db.NewCreateTable().
				Model(&m20251230100000_GameState{}).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create game_state table: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Down migration: drop the table
			_, err := db.NewDropTable().
				Model(&m20251230100000_GameState{}).
				IfExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop game_state table: %w", err)
			}

			return nil
		},
	)
}
