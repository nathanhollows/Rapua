package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type m20250921065956_InstanceSettings struct {
	bun.BaseModel `bun:"table:instance_settings"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	InstanceID       string                           `bun:"instance_id,pk,type:varchar(36)"`
	NavigationMode   m20241209083639_NavigationMode   `bun:"navigation_mode,type:int"`
	NavigationMethod m20241209083639_NavigationMethod `bun:"navigation_method,type:int"`
	MaxNextLocations int                              `bun:"max_next_locations,type:int,default:3"`
	// CompletionMethod  m20241209083639_CompletionMethod `bun:"completion_method,type:int"` // DROPPED
	MustCheckOut      bool `bun:"must_check_out,type:bool"` // ADDED
	ShowTeamCount     bool `bun:"show_team_count,type:bool"`
	EnablePoints      bool `bun:"enable_points,type:bool"`
	EnableBonusPoints bool `bun:"enable_bonus_points,type:bool"`
	ShowLeaderboard   bool `bun:"show_leaderboard,type:bool"`
}

type m20250921065956_Location struct {
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
	// Completion   m20241209083639_CompletionMethod `bun:"completion,type:int"` // DROPPED
	Points int `bun:"points,"`

	Clues    []m20241209083639_Clue   `bun:"rel:has-many,join:id=location_id"`
	Instance m20241209083639_Instance `bun:"rel:has-one,join:instance_id=id"`
	Marker   m20241209083639_Marker   `bun:"rel:has-one,join:marker_id=code"`
	Blocks   []m20241209083639_Block  `bun:"rel:has-many,join:id=location_id"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Drop completion_method from instance_settings and add must_check_out
		_, err := db.ExecContext(ctx, `
			ALTER TABLE instance_settings
			ADD COLUMN must_check_out BOOLEAN DEFAULT FALSE;
		`)
		if err != nil {
			return err
		}

		// Set must_check_out based on completion_method values
		_, err = db.ExecContext(ctx, `
			UPDATE instance_settings
			SET must_check_out = CASE
				WHEN completion_method = 1 THEN TRUE
				ELSE FALSE
			END;
		`)
		if err != nil {
			return err
		}

		// Drop completion_method column
		_, err = db.ExecContext(ctx, `
			ALTER TABLE instance_settings
			DROP COLUMN completion_method;
		`)
		if err != nil {
			return err
		}

		// Drop completion from locations (no data migration needed as field was unused)
		_, err = db.ExecContext(ctx, `
			ALTER TABLE locations
			DROP COLUMN completion;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Rollback: Add completion_method back to instance_settings and drop must_check_out
		_, err := db.ExecContext(ctx, `
			ALTER TABLE instance_settings
			ADD COLUMN completion_method INTEGER DEFAULT 0;
		`)
		if err != nil {
			return err
		}

		// Set completion_method based on must_check_out values
		_, err = db.ExecContext(ctx, `
			UPDATE instance_settings
			SET completion_method = CASE
				WHEN must_check_out = TRUE THEN 1
				ELSE 0
			END;
		`)
		if err != nil {
			return err
		}

		// Drop must_check_out column
		_, err = db.ExecContext(ctx, `
			ALTER TABLE instance_settings
			DROP COLUMN must_check_out;
		`)
		if err != nil {
			return err
		}

		// Add completion back to locations
		_, err = db.ExecContext(ctx, `
			ALTER TABLE locations
			ADD COLUMN completion INTEGER DEFAULT 0;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
