package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// getColumnNames returns the column names for a table using PRAGMA table_info.
func getColumnNames(ctx context.Context, db bun.IDB, tableName string) ([]string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// recreateTable performs the SQLite rename-create-copy-drop dance to add FK constraints.
// It queries the old table's column list and copies only columns that exist in both tables,
// which handles column order differences from ALTER TABLE ADD COLUMN migrations.
func recreateTable(ctx context.Context, db bun.IDB, tableName, newCreateSQL string) error {
	oldName := tableName + "_old"

	// Get column names from the existing table using raw SQL scan
	colNames, err := getColumnNames(ctx, db, tableName)
	if err != nil {
		return fmt.Errorf("getting columns for %s: %w", tableName, err)
	}

	// Rename old table
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, tableName, oldName)); err != nil {
		return fmt.Errorf("renaming %s: %w", tableName, err)
	}

	// Create new table with FK constraints
	if _, err := db.ExecContext(ctx, newCreateSQL); err != nil {
		return fmt.Errorf("creating new %s: %w", tableName, err)
	}

	// Get column names from the new table to find the intersection
	newColNames, err := getColumnNames(ctx, db, tableName)
	if err != nil {
		return fmt.Errorf("getting new columns for %s: %w", tableName, err)
	}

	// Build intersection of columns (only copy columns that exist in both)
	newColSet := make(map[string]bool, len(newColNames))
	for _, name := range newColNames {
		newColSet[name] = true
	}
	var quotedCols []string
	for _, name := range colNames {
		if newColSet[name] {
			quotedCols = append(quotedCols, fmt.Sprintf(`"%s"`, name))
		}
	}
	colList := strings.Join(quotedCols, ", ")

	// Copy data using explicit column list
	copySQL := fmt.Sprintf(`INSERT INTO "%s" (%s) SELECT %s FROM "%s"`, tableName, colList, colList, oldName)
	if _, err := db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("copying data to %s: %w\nSQL: %s", tableName, err, copySQL)
	}

	// Drop old table
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE "%s"`, oldName)); err != nil {
		return fmt.Errorf("dropping %s: %w", oldName, err)
	}

	return nil
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Must disable FK checks during table recreation to avoid
			// constraint violations while copying data between old/new tables.
			if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
				return fmt.Errorf("disabling foreign keys: %w", err)
			}
			defer func() {
				db.ExecContext(ctx, "PRAGMA foreign_keys=ON") //nolint:errcheck,gosec // G104: deferred cleanup, error intentionally ignored
			}()

			// Clean up orphaned rows that accumulated while FK constraints
			// were not enforced. These reference parents that no longer exist.
			orphanCleanups := []string{
				// check_ins referencing deleted teams or instances
				`DELETE FROM "check_ins" WHERE team_code NOT IN (SELECT code FROM teams)`,
				`DELETE FROM "check_ins" WHERE instance_id NOT IN (SELECT id FROM instances)`,
				// team_block_states referencing deleted teams or blocks
				`DELETE FROM "team_block_states" WHERE team_code NOT IN (SELECT code FROM teams)`,
				`DELETE FROM "team_block_states" WHERE block_id NOT IN (SELECT id FROM blocks)`,
				// notifications referencing deleted teams
				`DELETE FROM "notifications" WHERE team_code IS NOT NULL AND team_code NOT IN (SELECT code FROM teams)`,
				// uploads referencing deleted blocks
				`DELETE FROM "uploads" WHERE block_id IS NOT NULL AND block_id NOT IN (SELECT id FROM blocks)`,
				// instance_settings referencing deleted instances
				`DELETE FROM "instance_settings" WHERE instance_id NOT IN (SELECT id FROM instances)`,
				// facilitator_tokens referencing deleted instances
				`DELETE FROM "facilitator_tokens" WHERE instance_id NOT IN (SELECT id FROM instances)`,
				// share_links referencing deleted instances
				`DELETE FROM "share_links" WHERE template_id NOT IN (SELECT id FROM instances)`,
				// instances referencing deleted users
				`DELETE FROM "instances" WHERE user_id NOT IN (SELECT id FROM users)`,
			}
			for _, q := range orphanCleanups {
				if _, err := db.ExecContext(ctx, q); err != nil {
					return fmt.Errorf("cleaning orphaned rows: %w\nSQL: %s", err, q)
				}
			}

			// Order matters: recreate parent tables before children so FK
			// references point to the final (new) version of each parent.

			// --- instances (parent of many tables, child of users) ---
			err := recreateTable(ctx, db, "instances", `CREATE TABLE "instances" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"name" VARCHAR(255),
				"user_id" VARCHAR(36) REFERENCES "users"("id") ON DELETE CASCADE,
				"is_template" BOOL,
				"template_id" VARCHAR(36) REFERENCES "instances"("id") ON DELETE SET NULL,
				"start_time" DATETIME,
				"end_time" DATETIME,
				"is_quick_start_dismissed" BOOL,
				"game_structure" TEXT
			)`)
			if err != nil {
				return err
			}

			// --- credit_purchases (parent of credit_adjustments) ---
			err = recreateTable(ctx, db, "credit_purchases", `CREATE TABLE "credit_purchases" (
				"id" VARCHAR(36) UNIQUE NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"user_id" VARCHAR(36) NOT NULL REFERENCES "users"("id") ON DELETE CASCADE,
				"credits" INT NOT NULL,
				"amount_paid" INT NOT NULL,
				"stripe_payment_id" VARCHAR(255),
				"stripe_session_id" VARCHAR(255) NOT NULL UNIQUE,
				"stripe_customer_id" VARCHAR(255),
				"receipt_url" VARCHAR(500),
				"status" VARCHAR(20) NOT NULL DEFAULT 'pending'
			)`)
			if err != nil {
				return err
			}

			// --- instance_settings ---
			err = recreateTable(ctx, db, "instance_settings", `CREATE TABLE "instance_settings" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"instance_id" VARCHAR(36) PRIMARY KEY REFERENCES "instances"("id") ON DELETE CASCADE,
				"show_team_count" BOOL,
				"enable_points" BOOL,
				"enable_bonus_points" BOOL,
				"show_leaderboard" BOOL,
				"must_check_out" BOOL
			)`)
			if err != nil {
				return err
			}

			// --- locations ---
			err = recreateTable(ctx, db, "locations", `CREATE TABLE "locations" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"name" VARCHAR(255),
				"instance_id" VARCHAR(36) NOT NULL REFERENCES "instances"("id") ON DELETE CASCADE,
				"marker_id" VARCHAR(36) NOT NULL REFERENCES "markers"("code") ON DELETE RESTRICT,
				"criteria" VARCHAR(255),
				"order" INT,
				"total_visits" INT,
				"current_count" INT,
				"avg_duration" FLOAT,
				"points" INT,
				"slug" TEXT
			)`)
			if err != nil {
				return err
			}
			// Recreate the unique partial index on location slugs
			if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS "locations_instance_slug" ON "locations" ("instance_id", "slug") WHERE "slug" != ''`); err != nil {
				return fmt.Errorf("creating locations slug index: %w", err)
			}

			// --- teams ---
			err = recreateTable(ctx, db, "teams", `CREATE TABLE "teams" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36),
				"code" VARCHAR UNIQUE NOT NULL PRIMARY KEY,
				"name" VARCHAR,
				"instance_id" VARCHAR(36) NOT NULL REFERENCES "instances"("id") ON DELETE CASCADE,
				"has_started" BOOL DEFAULT false,
				"must_scan_out" VARCHAR,
				"points" INT,
				"skipped_group_ids" TEXT
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_teams_id" ON "teams" ("id")`); err != nil {
				return fmt.Errorf("creating teams id index: %w", err)
			}

			// --- blocks ---
			// owner_id is polymorphic: location_id for content blocks,
			// instance_id for start/finish blocks. A single FK reference
			// cannot cover both, so cascade deletion of blocks is handled
			// explicitly in the application (DeleteService).
			err = recreateTable(ctx, db, "blocks", `CREATE TABLE "blocks" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"owner_id" VARCHAR(36) NOT NULL,
				"type" VARCHAR(50),
				"context" VARCHAR(50),
				"data" JSONB,
				"ordering" INT,
				"points" INT,
				"validation_required" BOOL
			)`)
			if err != nil {
				return err
			}

			// --- check_ins ---
			err = recreateTable(ctx, db, "check_ins", `CREATE TABLE "check_ins" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"instance_id" VARCHAR(36) NOT NULL REFERENCES "instances"("id") ON DELETE CASCADE,
				"team_code" VARCHAR NOT NULL REFERENCES "teams"("code") ON DELETE CASCADE,
				"location_id" VARCHAR(36) NOT NULL REFERENCES "locations"("id") ON DELETE CASCADE,
				"time_in" DATETIME,
				"time_out" DATETIME,
				"must_check_out" BOOL,
				"points" INT,
				"blocks_completed" INT,
				PRIMARY KEY ("team_code", "location_id")
			)`)
			if err != nil {
				return err
			}

			// --- team_block_states ---
			err = recreateTable(ctx, db, "team_block_states", `CREATE TABLE "team_block_states" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"team_code" VARCHAR NOT NULL REFERENCES "teams"("code") ON DELETE CASCADE,
				"block_id" VARCHAR(36) NOT NULL REFERENCES "blocks"("id") ON DELETE CASCADE,
				"is_complete" BOOL,
				"points_awarded" INT,
				"player_data" JSONB,
				PRIMARY KEY ("team_code", "block_id")
			)`)
			if err != nil {
				return err
			}

			// --- notifications ---
			err = recreateTable(ctx, db, "notifications", `CREATE TABLE "notifications" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"content" VARCHAR(255),
				"type" VARCHAR(255),
				"team_code" VARCHAR(36) REFERENCES "teams"("code") ON DELETE CASCADE,
				"dismissed" BOOL
			)`)
			if err != nil {
				return err
			}

			// --- uploads ---
			err = recreateTable(ctx, db, "uploads", `CREATE TABLE "uploads" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"original_url" VARCHAR NOT NULL,
				"timestamp" DATETIME,
				"location_id" VARCHAR(36) REFERENCES "locations"("id") ON DELETE SET NULL,
				"instance_id" VARCHAR(36) REFERENCES "instances"("id") ON DELETE CASCADE,
				"team_code" VARCHAR REFERENCES "teams"("code") ON DELETE CASCADE,
				"block_id" VARCHAR(36) REFERENCES "blocks"("id") ON DELETE SET NULL,
				"storage" VARCHAR NOT NULL,
				"delete_data" VARCHAR,
				"type" VARCHAR
			)`)
			if err != nil {
				return err
			}

			// --- credit_adjustments ---
			err = recreateTable(ctx, db, "credit_adjustments", `CREATE TABLE "credit_adjustments" (
				"id" VARCHAR(36) UNIQUE NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"user_id" VARCHAR(36) NOT NULL REFERENCES "users"("id") ON DELETE CASCADE,
				"credits" INT NOT NULL,
				"reason" VARCHAR(255) NOT NULL,
				"credit_purchase_id" VARCHAR(36) REFERENCES "credit_purchases"("id") ON DELETE SET NULL
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_adjustments_user_id" ON "credit_adjustments" ("user_id")`); err != nil {
				return fmt.Errorf("creating credit_adjustments user_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_adjustments_purchase_id" ON "credit_adjustments" ("credit_purchase_id")`); err != nil {
				return fmt.Errorf("creating credit_adjustments purchase_id index: %w", err)
			}

			// --- team_start_logs ---
			err = recreateTable(ctx, db, "team_start_logs", `CREATE TABLE "team_start_logs" (
				"id" VARCHAR(36) UNIQUE NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"user_id" VARCHAR(36) NOT NULL REFERENCES "users"("id") ON DELETE CASCADE,
				"instance_id" VARCHAR(36) NOT NULL,
				"team_id" VARCHAR(36) NOT NULL
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_team_start_log_user_id" ON "team_start_logs" ("user_id")`); err != nil {
				return fmt.Errorf("creating team_start_logs user_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_team_start_log_instance_id" ON "team_start_logs" ("instance_id")`); err != nil {
				return fmt.Errorf("creating team_start_logs instance_id index: %w", err)
			}

			// Recreate indexes on credit_purchases (dropped during table recreation)
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_purchases_user_id" ON "credit_purchases" ("user_id")`); err != nil {
				return fmt.Errorf("creating credit_purchases user_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_purchases_stripe_session_id" ON "credit_purchases" ("stripe_session_id")`); err != nil {
				return fmt.Errorf("creating credit_purchases stripe_session_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_purchases_status" ON "credit_purchases" ("status")`); err != nil {
				return fmt.Errorf("creating credit_purchases status index: %w", err)
			}

			// --- facilitator_tokens ---
			err = recreateTable(ctx, db, "facilitator_tokens", `CREATE TABLE "facilitator_tokens" (
				"token" VARCHAR NOT NULL PRIMARY KEY,
				"instance_id" VARCHAR(36) NOT NULL REFERENCES "instances"("id") ON DELETE CASCADE,
				"locations" TEXT,
				"expires_at" DATETIME
			)`)
			if err != nil {
				return err
			}

			// --- share_links ---
			err = recreateTable(ctx, db, "share_links", `CREATE TABLE "share_links" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"template_id" VARCHAR(36) NOT NULL REFERENCES "instances"("id") ON DELETE CASCADE,
				"user_id" VARCHAR(36) REFERENCES "users"("id") ON DELETE SET NULL,
				"created_at" DATETIME,
				"expires_at" DATETIME,
				"max_uses" INT DEFAULT 0,
				"used_count" INT DEFAULT 0,
				"regenerate_codes" BOOL
			)`)
			if err != nil {
				return err
			}

			// Index on blocks.owner_id is beneficial for lookups even though
			// owner_id has no FK constraint (it is polymorphic).
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_blocks_owner_id" ON "blocks" ("owner_id")`); err != nil {
				return fmt.Errorf("creating blocks owner_id index: %w", err)
			}

			// --- Add indexes on FK columns for cascade performance ---
			fkIndexes := []struct {
				name  string
				table string
				col   string
			}{
				{"idx_instances_user_id", "instances", "user_id"},
				{"idx_instances_template_id", "instances", "template_id"},
				{"idx_locations_instance_id", "locations", "instance_id"},
				{"idx_locations_marker_id", "locations", "marker_id"},
				{"idx_teams_instance_id", "teams", "instance_id"},
				{"idx_check_ins_instance_id", "check_ins", "instance_id"},
				{"idx_notifications_team_code", "notifications", "team_code"},
				{"idx_uploads_instance_id", "uploads", "instance_id"},
				{"idx_uploads_team_code", "uploads", "team_code"},
				{"idx_uploads_block_id", "uploads", "block_id"},
				{"idx_uploads_location_id", "uploads", "location_id"},
				{"idx_facilitator_tokens_instance_id", "facilitator_tokens", "instance_id"},
				{"idx_share_links_template_id", "share_links", "template_id"},
				{"idx_share_links_user_id", "share_links", "user_id"},
			}

			for _, idx := range fkIndexes {
				q := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "%s" ON "%s" ("%s")`, idx.name, idx.table, idx.col)
				if _, err := db.ExecContext(ctx, q); err != nil {
					return fmt.Errorf("creating index %s: %w", idx.name, err)
				}
			}

			// Verify FK integrity
			rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
			if err != nil {
				return fmt.Errorf("checking foreign key integrity: %w", err)
			}
			defer rows.Close()
			if rows.Next() {
				var table, rowid, parent, fkid string
				_ = rows.Scan(&table, &rowid, &parent, &fkid)
				return fmt.Errorf(
					"foreign key violation: table=%s rowid=%s parent=%s fkid=%s",
					table,
					rowid,
					parent,
					fkid,
				)
			}

			return nil
		},

		// Down migration: recreate tables without FK constraints
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
				return fmt.Errorf("disabling foreign keys: %w", err)
			}
			defer func() {
				db.ExecContext(ctx, "PRAGMA foreign_keys=ON") //nolint:errcheck,gosec // G104: deferred cleanup, error intentionally ignored
			}()

			// Drop FK indexes added by up migration
			dropIndexes := []string{
				"idx_instances_user_id",
				"idx_instances_template_id",
				"idx_locations_instance_id",
				"idx_locations_marker_id",
				"idx_teams_instance_id",
				"idx_check_ins_instance_id",
				"idx_notifications_team_code",
				"idx_uploads_instance_id",
				"idx_uploads_team_code",
				"idx_uploads_block_id",
				"idx_uploads_location_id",
				"idx_facilitator_tokens_instance_id",
				"idx_share_links_template_id",
				"idx_share_links_user_id",
			}
			for _, idx := range dropIndexes {
				if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, idx)); err != nil {
					return fmt.Errorf("dropping index %s: %w", idx, err)
				}
			}

			// Recreate tables without FK constraints (reverse order: children first)
			err := recreateTable(ctx, db, "share_links", `CREATE TABLE "share_links" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"template_id" VARCHAR(36) NOT NULL,
				"user_id" VARCHAR(36),
				"created_at" DATETIME,
				"expires_at" DATETIME,
				"max_uses" INT DEFAULT 0,
				"used_count" INT DEFAULT 0,
				"regenerate_codes" BOOL
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "facilitator_tokens", `CREATE TABLE "facilitator_tokens" (
				"token" VARCHAR NOT NULL PRIMARY KEY,
				"instance_id" VARCHAR(36) NOT NULL,
				"locations" TEXT,
				"expires_at" DATETIME
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "team_start_logs", `CREATE TABLE "team_start_logs" (
				"id" VARCHAR(36) UNIQUE NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"user_id" VARCHAR(36) NOT NULL,
				"instance_id" VARCHAR(36) NOT NULL,
				"team_id" VARCHAR(36) NOT NULL
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_team_start_log_user_id" ON "team_start_logs" ("user_id")`); err != nil {
				return fmt.Errorf("creating team_start_logs user_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_team_start_log_instance_id" ON "team_start_logs" ("instance_id")`); err != nil {
				return fmt.Errorf("creating team_start_logs instance_id index: %w", err)
			}

			err = recreateTable(ctx, db, "credit_adjustments", `CREATE TABLE "credit_adjustments" (
				"id" VARCHAR(36) UNIQUE NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"user_id" VARCHAR(36) NOT NULL,
				"credits" INT NOT NULL,
				"reason" VARCHAR(255) NOT NULL,
				"credit_purchase_id" VARCHAR(36)
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_adjustments_user_id" ON "credit_adjustments" ("user_id")`); err != nil {
				return fmt.Errorf("creating credit_adjustments user_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_adjustments_purchase_id" ON "credit_adjustments" ("credit_purchase_id")`); err != nil {
				return fmt.Errorf("creating credit_adjustments purchase_id index: %w", err)
			}

			err = recreateTable(ctx, db, "uploads", `CREATE TABLE "uploads" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"original_url" VARCHAR NOT NULL,
				"timestamp" DATETIME,
				"location_id" VARCHAR(36),
				"instance_id" VARCHAR(36),
				"team_code" VARCHAR,
				"block_id" VARCHAR(36),
				"storage" VARCHAR NOT NULL,
				"delete_data" VARCHAR,
				"type" VARCHAR
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "notifications", `CREATE TABLE "notifications" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"content" VARCHAR(255),
				"type" VARCHAR(255),
				"team_code" VARCHAR(36),
				"dismissed" BOOL
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "team_block_states", `CREATE TABLE "team_block_states" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"team_code" VARCHAR NOT NULL,
				"block_id" VARCHAR(36) NOT NULL,
				"is_complete" BOOL,
				"points_awarded" INT,
				"player_data" JSONB,
				PRIMARY KEY ("team_code", "block_id")
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "check_ins", `CREATE TABLE "check_ins" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"instance_id" VARCHAR(36) NOT NULL,
				"team_code" VARCHAR NOT NULL,
				"location_id" VARCHAR(36) NOT NULL,
				"time_in" DATETIME,
				"time_out" DATETIME,
				"must_check_out" BOOL,
				"points" INT,
				"blocks_completed" INT,
				PRIMARY KEY ("team_code", "location_id")
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "blocks", `CREATE TABLE "blocks" (
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"owner_id" VARCHAR(36) NOT NULL,
				"type" VARCHAR(50),
				"context" VARCHAR(50),
				"data" JSONB,
				"ordering" INT,
				"points" INT,
				"validation_required" BOOL
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "teams", `CREATE TABLE "teams" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36),
				"code" VARCHAR UNIQUE NOT NULL PRIMARY KEY,
				"name" VARCHAR,
				"instance_id" VARCHAR(36) NOT NULL,
				"has_started" BOOL DEFAULT false,
				"must_scan_out" VARCHAR,
				"points" INT,
				"skipped_group_ids" TEXT
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_teams_id" ON "teams" ("id")`); err != nil {
				return fmt.Errorf("creating teams id index: %w", err)
			}

			err = recreateTable(ctx, db, "locations", `CREATE TABLE "locations" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"name" VARCHAR(255),
				"instance_id" VARCHAR(36) NOT NULL,
				"marker_id" VARCHAR(36) NOT NULL,
				"criteria" VARCHAR(255),
				"order" INT,
				"total_visits" INT,
				"current_count" INT,
				"avg_duration" FLOAT,
				"points" INT,
				"slug" TEXT
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS "locations_instance_slug" ON "locations" ("instance_id", "slug") WHERE "slug" != ''`); err != nil {
				return fmt.Errorf("creating locations slug index: %w", err)
			}

			err = recreateTable(ctx, db, "instance_settings", `CREATE TABLE "instance_settings" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"instance_id" VARCHAR(36) PRIMARY KEY,
				"show_team_count" BOOL,
				"enable_points" BOOL,
				"enable_bonus_points" BOOL,
				"show_leaderboard" BOOL,
				"must_check_out" BOOL
			)`)
			if err != nil {
				return err
			}

			err = recreateTable(ctx, db, "credit_purchases", `CREATE TABLE "credit_purchases" (
				"id" VARCHAR(36) UNIQUE NOT NULL PRIMARY KEY,
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"user_id" VARCHAR(36) NOT NULL,
				"credits" INT NOT NULL,
				"amount_paid" INT NOT NULL,
				"stripe_payment_id" VARCHAR(255),
				"stripe_session_id" VARCHAR(255) NOT NULL UNIQUE,
				"stripe_customer_id" VARCHAR(255),
				"receipt_url" VARCHAR(500),
				"status" VARCHAR(20) NOT NULL DEFAULT 'pending'
			)`)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_purchases_user_id" ON "credit_purchases" ("user_id")`); err != nil {
				return fmt.Errorf("creating credit_purchases user_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_purchases_stripe_session_id" ON "credit_purchases" ("stripe_session_id")`); err != nil {
				return fmt.Errorf("creating credit_purchases stripe_session_id index: %w", err)
			}
			if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "idx_credit_purchases_status" ON "credit_purchases" ("status")`); err != nil {
				return fmt.Errorf("creating credit_purchases status index: %w", err)
			}

			err = recreateTable(ctx, db, "instances", `CREATE TABLE "instances" (
				"created_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"updated_at" DATETIME NOT NULL DEFAULT current_timestamp,
				"id" VARCHAR(36) NOT NULL PRIMARY KEY,
				"name" VARCHAR(255),
				"user_id" VARCHAR(36),
				"is_template" BOOL,
				"template_id" VARCHAR(36),
				"start_time" DATETIME,
				"end_time" DATETIME,
				"is_quick_start_dismissed" BOOL,
				"game_structure" TEXT
			)`)
			if err != nil {
				return err
			}

			return nil
		},
	)
}
