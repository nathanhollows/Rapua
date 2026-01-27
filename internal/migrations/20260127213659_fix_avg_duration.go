package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Recalculates location avg_duration to fix rolling average bug
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Get all unique instance IDs
		var instanceIDs []string
		err := db.NewSelect().
			Model((*m20260127120000_Location)(nil)).
			Column("instance_id").
			Distinct().
			Scan(ctx, &instanceIDs)
		if err != nil {
			return fmt.Errorf("20260127120000_fix_avg_duration.go: fetch instance IDs: %w", err)
		}

		// Recalculate statistics for each instance
		for _, instanceID := range instanceIDs {
			err := m20260127120000_updateLocationStatistics(ctx, db, instanceID)
			if err != nil {
				return fmt.Errorf("20260127120000_fix_avg_duration.go: update stats for instance %s: %w", instanceID, err)
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration - no action needed
		// The old (incorrect) avg_duration values cannot be restored
		return nil
	})
}

// Timestamped copy of Location model for this migration
type m20260127120000_Location struct {
	bun.BaseModel `bun:"table:locations"`

	ID         string `bun:"id,pk,notnull"`
	InstanceID string `bun:"instance_id,notnull"`
}

// Timestamped copy of CheckIn model for this migration
type m20260127120000_CheckIn struct {
	bun.BaseModel `bun:"table:check_ins"`

	ID         string `bun:"id,pk"`
	LocationID string `bun:"location_id"`
	InstanceID string `bun:"instance_id"`
}

// Recalculates location statistics using the correct formula
func m20260127120000_updateLocationStatistics(ctx context.Context, db *bun.DB, instanceID string) error {
	// Subquery: Count unique teams for each location
	totalVisitsSubquery := db.NewSelect().
		TableExpr("check_ins AS check_in").
		ColumnExpr("COUNT(DISTINCT team_code)").
		Where("check_in.location_id = location.id").
		Where("check_in.instance_id = location.instance_id")

	// Subquery: Count currently checked-in teams
	currentCountSubquery := db.NewSelect().
		TableExpr("check_ins AS check_in").
		ColumnExpr("COUNT(*)").
		Where("check_in.location_id = location.id").
		Where("check_in.instance_id = location.instance_id").
		Where("check_in.time_out IS NULL")

	// Subquery: Compute average duration in seconds (ignoring zero time_out values)
	// Use julianday for proper date arithmetic, multiply by 86400 to convert days to seconds
	// TimeOut is time.Time (not nullable), so zero value is '0001-01-01', check for year > 1000
	avgDurationSubquery := db.NewSelect().
		TableExpr("check_ins AS check_in").
		ColumnExpr("COALESCE(AVG((julianday(time_out) - julianday(time_in)) * 86400), 0)").
		Where("check_in.location_id = location.id").
		Where("check_in.instance_id = location.instance_id").
		Where("strftime('%Y', check_in.time_out) > '1000'") // Ignore zero time values

	query := db.NewUpdate().
		TableExpr("locations AS location").
		Set("total_visits = (?)", totalVisitsSubquery).
		Set("current_count = (?)", currentCountSubquery).
		Set("avg_duration = (?)", avgDurationSubquery).
		Where("instance_id = ?", instanceID)

	_, err := query.Exec(ctx)

	return err
}
