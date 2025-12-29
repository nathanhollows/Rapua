package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type m20250224085041_ShareLink struct {
	bun.BaseModel `bun:"table:share_links"`

	ID              string       `bun:"id,pk,type:varchar(36)"`
	TemplateID      string       `bun:"template_id,type:varchar(36),notnull"`
	UserID          string       `bun:"user_id,type:varchar(36)"` // Owner of the link
	CreatedAt       time.Time    `bun:"created_at,nullzero"`
	ExpiresAt       bun.NullTime `bun:"expires_at,nullzero"`
	MaxUses         int          `bun:"max_uses,type:int,default:0"` // 0 means unlimited
	UsedCount       int          `bun:"used_count,type:int,default:0"`
	RegenerateCodes bool         `bun:"regenerate_codes,type:bool"`

	Template *m20250219013821_Instance `bun:"rel:belongs-to,join:template_id=id"`
}

func init() {
	Migrations.MustRegister(func(_ context.Context, db *bun.DB) error {
		_, err := db.NewCreateTable().Model(&m20250224085041_ShareLink{}).IfNotExists().Exec(context.Background())
		return err
	}, func(_ context.Context, db *bun.DB) error {
		_, err := db.NewDropTable().Model(&m20250224085041_ShareLink{}).IfExists().Exec(context.Background())
		return err
	})
}
