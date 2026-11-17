package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260901010000_up, m20260901010000_down)
}

// m20260901010000_up exists because an unregistered broker is silently skipped,
// opening any gate it held, and its export will not import back.
//
// Dependents are cleared explicitly: SQLite enforces foreign keys per
// connection, and a migration should not depend on how one is configured.
func m20260901010000_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "blocks") {
		return nil
	}

	const brokerIDs = `SELECT "id" FROM "blocks" WHERE "type" = 'broker'`

	// An upload is the player's file, so it outlives its block.
	if tableExists(ctx, db, "uploads") {
		if _, err := db.ExecContext(
			ctx, `UPDATE "uploads" SET "block_id" = NULL WHERE "block_id" IN (`+brokerIDs+`)`,
		); err != nil {
			return fmt.Errorf("detaching uploads from broker blocks: %w", err)
		}
	}

	if tableExists(ctx, db, "run_block_states") {
		if _, err := db.ExecContext(
			ctx, `DELETE FROM "run_block_states" WHERE "block_id" IN (`+brokerIDs+`)`,
		); err != nil {
			return fmt.Errorf("deleting broker block states: %w", err)
		}
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM "blocks" WHERE "type" = 'broker'`); err != nil {
		return fmt.Errorf("deleting broker blocks: %w", err)
	}

	return nil
}

// m20260901010000_down is a no-op: there is nothing to rebuild from.
func m20260901010000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
