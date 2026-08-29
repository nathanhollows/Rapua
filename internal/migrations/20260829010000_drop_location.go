package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260829010000_up, m20260829010000_down)
}

// m20260829010000_up drops Location and Marker entirely. Safe only because
// 20260829000000_finish_locations_to_objectives.go guarantees, before this can
// run, that every Location is represented by an Objective (it fails loudly
// otherwise): including each Location's Marker code, already copied as a
// literal string into that Objective's synthetic scan block, so Marker's
// deletion here needs no special handling of its own.
func m20260829010000_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "locations") {
		return nil
	}

	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return fmt.Errorf("disabling foreign keys: %w", err)
	}
	defer func() {
		//nolint:errcheck,gosec // deferred cleanup, error intentionally ignored
		db.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	}()

	// check_ins: the whole check-in mechanic is gone, not just its Location
	// reference (Objectives are proved/revealed via blocks, not scan-in): drop
	// outright. History was explicitly confirmed out of scope to preserve.
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "check_ins"`); err != nil {
		return fmt.Errorf("dropping check_ins: %w", err)
	}

	// uploads: drop the location_id FK/column. No Objective equivalent exists,
	// and building one would be new feature work out of scope here. ALTER TABLE
	// DROP COLUMN is used instead of recreateTable's rename dance: SQLite
	// rewrites REFERENCES "runs" clauses in other tables when "runs" is
	// renamed, leaving run_block_states, notifications, run_var_states,
	// objective_context_completions, and uploads pointing at a dead "runs_old"
	// table. The column cannot be dropped while indexed.
	if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS "idx_uploads_location_id"`); err != nil {
		return fmt.Errorf("dropping uploads location_id index: %w", err)
	}
	// columnExists guard: DROP COLUMN errors on a column that's already gone,
	// which a retry after an earlier partial failure would otherwise hit.
	if columnExists(ctx, db, "uploads", "location_id") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "uploads" DROP COLUMN "location_id"`); err != nil {
			return fmt.Errorf("dropping uploads.location_id: %w", err)
		}
	}

	// runs: drop must_scan_out. The check-out mechanic (BlockingLocation,
	// QuestSettings.MustCheckOut) is Location-only with no Objective
	// equivalent, deliberately not ported.
	if columnExists(ctx, db, "runs", "must_scan_out") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "runs" DROP COLUMN "must_scan_out"`); err != nil {
			return fmt.Errorf("dropping runs.must_scan_out: %w", err)
		}
	}

	// quest_settings: drop must_check_out, the other half of the same
	// check-out mechanic.
	if columnExists(ctx, db, "quest_settings", "must_check_out") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "quest_settings" DROP COLUMN "must_check_out"`); err != nil {
			return fmt.Errorf("dropping quest_settings.must_check_out: %w", err)
		}
	}

	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "locations"`); err != nil {
		return fmt.Errorf("dropping locations: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS "markers"`); err != nil {
		return fmt.Errorf("dropping markers: %w", err)
	}

	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("checking foreign key integrity: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var table, rowid, parent, fkid string
		_ = rows.Scan(&table, &rowid, &parent, &fkid)
		return fmt.Errorf("foreign key violation: table=%s rowid=%s parent=%s fkid=%s", table, rowid, parent, fkid)
	}

	return nil
}

// m20260829010000_down restores schema only, not data: the dropped rows are
// gone. By the time this migration ran, every Location's content already
// existed as an Objective (m20260829000000 guarantees it), so a rollback of
// this migration is a schema-compatibility escape hatch, not a way to recover
// lost content.
func m20260829010000_down(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return fmt.Errorf("disabling foreign keys: %w", err)
	}
	defer func() {
		//nolint:errcheck,gosec // deferred cleanup, error intentionally ignored
		db.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	}()

	// markers before locations: locations.marker_id references markers(code).
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "markers" (
		"created_at" TIMESTAMP NOT NULL DEFAULT current_timestamp,
		"updated_at" TIMESTAMP NOT NULL DEFAULT current_timestamp,
		"code" VARCHAR NOT NULL,
		"lat" float,
		"lng" float,
		"name" varchar(255),
		"total_visits" int,
		"current_count" int,
		"avg_duration" float,
		PRIMARY KEY ("code"),
		UNIQUE ("code")
	)`); err != nil {
		return fmt.Errorf("recreating markers: %w", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "locations" (
		"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"id" VARCHAR(36) NOT NULL PRIMARY KEY,
		"name" VARCHAR(255),
		"quest_id" VARCHAR(36) NOT NULL REFERENCES "quests"("id") ON DELETE CASCADE,
		"marker_id" VARCHAR(36) NOT NULL REFERENCES "markers"("code") ON DELETE RESTRICT,
		"criteria" VARCHAR(255),
		"order" INT,
		"total_visits" INT,
		"current_count" INT,
		"avg_duration" FLOAT,
		"points" INT,
		"slug" TEXT,
		"when_clause" TEXT
	)`); err != nil {
		return fmt.Errorf("recreating locations: %w", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`CREATE UNIQUE INDEX IF NOT EXISTS "locations_instance_slug" ON "locations" ("quest_id", "slug") WHERE "slug" != ''`,
	); err != nil {
		return fmt.Errorf("recreating locations slug index: %w", err)
	}
	if _, err := db.ExecContext(
		ctx, `CREATE INDEX IF NOT EXISTS "idx_locations_instance_id" ON "locations" ("quest_id")`,
	); err != nil {
		return fmt.Errorf("recreating locations quest_id index: %w", err)
	}
	if _, err := db.ExecContext(
		ctx, `CREATE INDEX IF NOT EXISTS "idx_locations_marker_id" ON "locations" ("marker_id")`,
	); err != nil {
		return fmt.Errorf("recreating locations marker_id index: %w", err)
	}

	// ADD COLUMN restores the schema (including the FK constraint on
	// uploads.location_id, which SQLite permits when the default is NULL);
	// the data in those columns is gone for good.
	if !columnExists(ctx, db, "runs", "must_scan_out") {
		if _, err := db.ExecContext(ctx, `ALTER TABLE "runs" ADD COLUMN "must_scan_out" VARCHAR`); err != nil {
			return fmt.Errorf("restoring runs.must_scan_out: %w", err)
		}
	}

	if !columnExists(ctx, db, "quest_settings", "must_check_out") {
		if _, err := db.ExecContext(
			ctx, `ALTER TABLE "quest_settings" ADD COLUMN "must_check_out" BOOLEAN DEFAULT FALSE`,
		); err != nil {
			return fmt.Errorf("restoring quest_settings.must_check_out: %w", err)
		}
	}

	if !columnExists(ctx, db, "uploads", "location_id") {
		if _, err := db.ExecContext(
			ctx, `ALTER TABLE "uploads" ADD COLUMN "location_id" VARCHAR(36) REFERENCES "locations"("id") ON DELETE SET NULL`,
		); err != nil {
			return fmt.Errorf("restoring uploads.location_id: %w", err)
		}
	}
	if _, err := db.ExecContext(
		ctx, `CREATE INDEX IF NOT EXISTS "idx_uploads_location_id" ON "uploads" ("location_id")`,
	); err != nil {
		return fmt.Errorf("recreating uploads location_id index: %w", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "check_ins" (
		"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
		"quest_id" VARCHAR(36) NOT NULL REFERENCES "quests"("id") ON DELETE CASCADE,
		"run_code" VARCHAR NOT NULL REFERENCES "runs"("code") ON DELETE CASCADE,
		"location_id" VARCHAR(36) NOT NULL REFERENCES "locations"("id") ON DELETE CASCADE,
		"time_in" DATETIME,
		"time_out" DATETIME,
		"must_check_out" BOOL,
		"points" INT,
		"blocks_completed" INT,
		PRIMARY KEY ("run_code", "location_id")
	)`); err != nil {
		return fmt.Errorf("recreating check_ins: %w", err)
	}
	if _, err := db.ExecContext(
		ctx, `CREATE INDEX IF NOT EXISTS "idx_check_ins_instance_id" ON "check_ins" ("quest_id")`,
	); err != nil {
		return fmt.Errorf("recreating check_ins quest_id index: %w", err)
	}

	return nil
}
