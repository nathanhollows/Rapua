package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260901010000_up, m20260901010000_down)
}

// m20260901010000_up removes every broker block and the player state hanging
// off it.
//
// A surviving row fails in two ways, and the quiet one is the reason this
// exists. The type is no longer registered, so loading it is skipped: the block
// is simply absent from the page, and any gate it held opens, with nothing said
// either way. Export is the loud half but arrives late: it reads block rows
// without consulting the registry, so it still writes type "broker" into the
// document, and importing that document back fails on an unknown block type.
//
// The dependent rows are cleared explicitly rather than left to the foreign
// keys. SQLite enforces those per connection, so whether they fire depends on
// how the connection running this happens to be configured, and a migration
// should not.
func m20260901010000_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "blocks") {
		return nil
	}

	const brokerIDs = `SELECT "id" FROM "blocks" WHERE "type" = 'broker'`

	// An upload is a player's own file and outlives the block it was made
	// against, so it loses the reference rather than being deleted with it.
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

// m20260901010000_down does nothing. The blocks and their player state are
// gone, and there is nothing left to rebuild them from.
func m20260901010000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
