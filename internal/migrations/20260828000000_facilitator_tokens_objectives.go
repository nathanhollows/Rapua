package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Renames facilitator_tokens.locations to objectives: facilitator token
// scoping now targets objectives, not locations. A straight rename, not an
// additive column: the admin UI never had a location-picker for this field,
// so no real data depends on the old one.
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if !tableExists(ctx, db, "facilitator_tokens") {
				return nil
			}
			if !columnExists(ctx, db, "facilitator_tokens", "locations") {
				return nil
			}
			if _, err := db.ExecContext(
				ctx, `ALTER TABLE facilitator_tokens RENAME COLUMN locations TO objectives`,
			); err != nil {
				return fmt.Errorf("rename facilitator_tokens.locations to objectives: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			if !tableExists(ctx, db, "facilitator_tokens") {
				return nil
			}
			if !columnExists(ctx, db, "facilitator_tokens", "objectives") {
				return nil
			}
			if _, err := db.ExecContext(
				ctx, `ALTER TABLE facilitator_tokens RENAME COLUMN objectives TO locations`,
			); err != nil {
				return fmt.Errorf("reverse rename facilitator_tokens.objectives to locations: %w", err)
			}
			return nil
		},
	)
}
