package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type m20241209090041_Team struct {
	bun.BaseModel `bun:"table:teams"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	ID           string `bun:"id,type:varchar(36),pk"`
	Code         string `bun:"code"` // No longer unique
	Name         string `bun:"name"`
	InstanceID   string `bun:"instance_id,notnull"`
	HasStarted   bool   `bun:"has_started,default:false"`
	MustCheckOut string `bun:"must_scan_out"`
	Points       int    `bun:"points"`

	// Relationships with reference to previous migrations.
	Instance         m20241209083639_Instance         `bun:"rel:has-one,join:instance_id=id"`
	CheckIns         []m20241209083639_CheckIn        `bun:"rel:has-many,join:code=team_code"`
	BlockingLocation m20241209083639_Location         `bun:"rel:has-one,join:must_scan_out=marker_id,join:instance_id=instance_id"`
	Messages         []m20241209083639_Notification   `bun:"rel:has-many,join:code=team_code"`
	Blocks           []m20241209083639_TeamBlockState `bun:"rel:has-many,join:code=team_code"`
}

func init() {
	// Adds the ID field to the Team struct.
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Add the ID column.
		_, err := db.NewAddColumn().Model((*m20241209090041_Team)(nil)).ColumnExpr("id varchar(36)").Exec(ctx)
		if err != nil {
			return fmt.Errorf("add column id: %w", err)
		}

		// Update the ID column.
		type Team struct {
			ID   string `bun:"id,pk"`
			Code string `bun:"code"`
		}
		var teams []Team
		err = db.NewSelect().Column("code").Model((*m20241209090041_Team)(nil)).Scan(ctx, &teams)
		if err != nil {
			return fmt.Errorf("select teams: %w", err)
		}
		if len(teams) > 0 {
			for i, team := range teams {
				if team.ID == "" {
					teams[i].ID = uuid.New().String()
				}
			}
			values := db.NewValues(&teams)
			query := db.NewUpdate().
				With("_data", values).
				Model((*Team)(nil)).
				TableExpr("_data").
				Set("id = _data.id").
				Where("team.code = _data.code")
			_, err = query.Exec(ctx)
			if err != nil {
				return fmt.Errorf("20241209090041_teams_id.go: update id: %w", err)
			}
		}

		_, err = db.NewCreateIndex().Model((*m20241209090041_Team)(nil)).Index("id").Column("id").Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration.
		_, err := db.NewDropIndex().Model((*m20241209090041_Team)(nil)).Index("id").Exec(ctx)
		if err != nil {
			return fmt.Errorf("20241209090041_teams_id.go: drop index id: %w", err)
		}
		_, err = db.NewDropColumn().Model((*m20241209090041_Team)(nil)).Column("id").Exec(ctx)
		if err != nil {
			return fmt.Errorf("20241209090041_teams_id.go: drop column id: %w", err)
		}
		return nil
	})
}
