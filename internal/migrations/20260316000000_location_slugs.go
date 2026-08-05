package migrations

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// SQLite does not allow NOT NULL without DEFAULT in ALTER TABLE ADD COLUMN.
		// Use DEFAULT '' so existing rows satisfy the constraint during the alter.
		_, err := db.ExecContext(ctx, `ALTER TABLE locations ADD COLUMN slug TEXT DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("adding slug column: %w", err)
		}

		// Backfill slugs from existing location names, unique per instance.
		var locations []m20260316_Location
		if err = db.NewSelect().Model(&locations).Scan(ctx); err != nil {
			return fmt.Errorf("fetching locations: %w", err)
		}

		byInstance := make(map[string][]m20260316_Location)
		for _, loc := range locations {
			byInstance[loc.InstanceID] = append(byInstance[loc.InstanceID], loc)
		}

		for _, locs := range byInstance {
			used := make(map[string]bool)
			for _, loc := range locs {
				base := m20260316_slugify(loc.Name)
				if base == "" {
					base = loc.ID[:8]
				}
				candidate := base
				for n := 2; used[candidate]; n++ {
					candidate = fmt.Sprintf("%s-%d", base, n)
				}
				used[candidate] = true
				if _, err = db.ExecContext(
					ctx,
					`UPDATE locations SET slug = ? WHERE id = ?`,
					candidate,
					loc.ID,
				); err != nil {
					return fmt.Errorf("setting slug for location %s: %w", loc.ID, err)
				}
			}
		}

		// Partial unique index: only enforce uniqueness on non-empty slugs so that
		// test fixtures inserting rows without slugs do not violate the constraint.
		_, err = db.ExecContext(
			ctx,
			`CREATE UNIQUE INDEX locations_instance_slug `+
				`ON locations (instance_id, slug) WHERE slug != ''`,
		)
		if err != nil {
			return fmt.Errorf("creating slug index: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS locations_instance_slug`); err != nil {
			return fmt.Errorf("dropping slug index: %w", err)
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE locations DROP COLUMN slug`); err != nil {
			return fmt.Errorf("dropping slug column: %w", err)
		}
		return nil
	})
}

type m20260316_Location struct {
	bun.BaseModel `       bun:"table:locations"`
	ID            string `bun:"id,pk"`
	Name          string `bun:"name"`
	InstanceID    string `bun:"instance_id"`
}

var m20260316_nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func m20260316_slugify(s string) string {
	s = strings.ToLower(s)
	s = m20260316_nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
