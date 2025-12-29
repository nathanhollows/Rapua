package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type m20250202083758_FacilitatorTokens struct {
	bun.BaseModel `bun:"table:facilitator_tokens"`

	Token      string    `bun:"token,pk"`
	InstanceID string    `bun:"instance_id,notnull"`
	Locations  []string  `bun:"locations,type:text"`
	ExpiresAt  time.Time `bun:"expires_at,type:datetime"`
}

func init() {
	Migrations.MustRegister(
		func(_ context.Context, db *bun.DB) error {
			_, err := db.NewCreateTable().
				Model(&m20250202083758_FacilitatorTokens{}).
				IfNotExists().
				Exec(context.Background())
			return err
		}, func(_ context.Context, db *bun.DB) error {
			_, err := db.NewDropTable().
				Model(&m20250202083758_FacilitatorTokens{}).
				IfExists().
				Exec(context.Background())
			return err
		})
}
