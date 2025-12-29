package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

func init() {
	type MediaType string

	type Upload struct {
		bun.BaseModel `bun:"table:uploads,alias:u"`

		ID          string    `bun:"id,pk,notnull"`
		OriginalURL string    `bun:"original_url,notnull"` // Original file link
		Timestamp   time.Time `bun:"timestamp"`
		LocationID  string    `bun:"location_id,nullzero"`
		InstanceID  string    `bun:"instance_id,nullzero"`
		TeamCode    string    `bun:"team_code,nullzero"`
		BlockID     string    `bun:"block_id,nullzero"`
		Storage     string    `bun:"storage,notnull"`
		DeleteData  string    `bun:"delete_data"`
		Type        MediaType `bun:"type"`
		sizes       string    `bun:"sizes"` //nolint:unused // Historical migration field
	}

	Migrations.MustRegister(func(_ context.Context, db *bun.DB) error {
		_, err := db.NewCreateTable().Model(&Upload{}).IfNotExists().Exec(context.Background())
		return err
	}, func(_ context.Context, db *bun.DB) error {
		_, err := db.NewDropTable().Model(&Upload{}).IfExists().Exec(context.Background())
		return err
	})
}
