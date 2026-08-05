package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Renames the "instance" domain to "quest" and the "team" domain to "run",
// across both table names and the foreign-key columns that reference them.
func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			// Disable foreign keys during migration to avoid constraint issues
			if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("disable foreign keys: %w", err)
			}

			// Table renames
			tableRenames := [][2]string{
				{"instances", "quests"},
				{"instance_settings", "quest_settings"},
				{"teams", "runs"},
				{"team_var_states", "run_var_states"},
				{"team_start_logs", "run_start_logs"},
				{"team_block_states", "run_block_states"},
			}
			for _, pair := range tableRenames {
				old, newName := pair[0], pair[1]
				if !tableExists(ctx, db, old) {
					continue // table doesn't exist, nothing to rename
				}
				if tableExists(ctx, db, newName) {
					continue // new table already exists, skip
				}
				sql := fmt.Sprintf("ALTER TABLE %q RENAME TO %q", old, newName)
				if _, err := db.ExecContext(ctx, sql); err != nil {
					return fmt.Errorf("rename table %s → %s: %w", old, newName, err)
				}
			}

			// Column renames. Skip tables that don't exist (some tables are
			// only created in later migrations or production environments).
			columnMigrations := []struct {
				table string
				cols  [][2]string // {old, new}
			}{
				{"runs", [][2]string{{"instance_id", "quest_id"}}},
				{"locations", [][2]string{{"instance_id", "quest_id"}}},
				{"check_ins", [][2]string{{"instance_id", "quest_id"}, {"team_code", "run_code"}}},
				{"run_var_states", [][2]string{{"instance_id", "quest_id"}, {"team_code", "run_code"}}},
				{"run_start_logs", [][2]string{{"instance_id", "quest_id"}, {"team_id", "run_id"}}},
				{"run_block_states", [][2]string{{"team_code", "run_code"}}},
				{"notifications", [][2]string{{"team_code", "run_code"}}},
				{"uploads", [][2]string{{"instance_id", "quest_id"}, {"team_code", "run_code"}}},
				{"quest_settings", [][2]string{{"instance_id", "quest_id"}}},
				{"facilitator_tokens", [][2]string{{"instance_id", "quest_id"}}},
			}

			for _, cm := range columnMigrations {
				if !tableExists(ctx, db, cm.table) {
					continue
				}
				for _, col := range cm.cols {
					if !columnExists(ctx, db, cm.table, col[0]) {
						continue // column already renamed or doesn't exist
					}
					sql := fmt.Sprintf("ALTER TABLE %q RENAME COLUMN %q TO %q", cm.table, col[0], col[1])
					if _, err := db.ExecContext(ctx, sql); err != nil {
						return fmt.Errorf("rename column %s.%s → %s: %w", cm.table, col[0], col[1], err)
					}
				}
			}

			// Add quest_id to run_block_states (new column not in original schema).
			if tableExists(ctx, db, "run_block_states") && !columnExists(ctx, db, "run_block_states", "quest_id") {
				if _, err := db.ExecContext(ctx,
					`ALTER TABLE run_block_states ADD COLUMN quest_id VARCHAR(36) NOT NULL DEFAULT ''`,
				); err != nil {
					return fmt.Errorf("add quest_id to run_block_states: %w", err)
				}
				if _, err := db.ExecContext(
					ctx,
					`UPDATE run_block_states SET quest_id = COALESCE((`+
						`SELECT quest_id FROM runs WHERE runs.code = run_block_states.run_code`+
						`), 'orphaned')`,
				); err != nil {
					return fmt.Errorf("populate quest_id in run_block_states: %w", err)
				}
			}

			if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("enable foreign keys: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Down migration: reverse column renames
			if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("disable foreign keys: %w", err)
			}

			// Drop the column the up path added, before the table is renamed back.
			if tableExists(ctx, db, "run_block_states") && columnExists(ctx, db, "run_block_states", "quest_id") {
				if _, err := db.ExecContext(ctx, `ALTER TABLE run_block_states DROP COLUMN quest_id`); err != nil {
					return fmt.Errorf("drop quest_id from run_block_states: %w", err)
				}
			}

			reverseColumnMigrations := []struct {
				table string
				cols  [][2]string // {new, old} — reversed
			}{
				{"runs", [][2]string{{"quest_id", "instance_id"}}},
				{"locations", [][2]string{{"quest_id", "instance_id"}}},
				{"check_ins", [][2]string{{"quest_id", "instance_id"}, {"run_code", "team_code"}}},
				{"run_var_states", [][2]string{{"quest_id", "instance_id"}, {"run_code", "team_code"}}},
				{"run_start_logs", [][2]string{{"quest_id", "instance_id"}, {"run_id", "team_id"}}},
				{"run_block_states", [][2]string{{"run_code", "team_code"}}},
				{"notifications", [][2]string{{"run_code", "team_code"}}},
				{"uploads", [][2]string{{"quest_id", "instance_id"}, {"run_code", "team_code"}}},
				{"quest_settings", [][2]string{{"quest_id", "instance_id"}}},
				{"facilitator_tokens", [][2]string{{"quest_id", "instance_id"}}},
			}
			// Guards mirror the up path: the up migration is not transactional, so a
			// partial run must still be reversible.
			for _, cm := range reverseColumnMigrations {
				if !tableExists(ctx, db, cm.table) {
					continue
				}
				for _, col := range cm.cols {
					if !columnExists(ctx, db, cm.table, col[0]) {
						continue
					}
					sql := fmt.Sprintf("ALTER TABLE %q RENAME COLUMN %q TO %q", cm.table, col[0], col[1])
					if _, err := db.ExecContext(ctx, sql); err != nil {
						return fmt.Errorf("reverse column rename %s.%s → %s: %w", cm.table, col[0], col[1], err)
					}
				}
			}

			// Reverse table renames
			reverseTableRenames := [][2]string{
				{"quests", "instances"},
				{"quest_settings", "instance_settings"},
				{"runs", "teams"},
				{"run_var_states", "team_var_states"},
				{"run_start_logs", "team_start_logs"},
				{"run_block_states", "team_block_states"},
			}
			for _, pair := range reverseTableRenames {
				old, newName := pair[0], pair[1]
				if !tableExists(ctx, db, old) || tableExists(ctx, db, newName) {
					continue
				}
				sql := fmt.Sprintf("ALTER TABLE %q RENAME TO %q", old, newName)
				if _, err := db.ExecContext(ctx, sql); err != nil {
					return fmt.Errorf("reverse table rename %s → %s: %w", old, newName, err)
				}
			}

			if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("enable foreign keys: %w", err)
			}
			return nil
		},
	)
}

// tableExists checks whether a table exists in the SQLite database.
func tableExists(ctx context.Context, db *bun.DB, name string) bool {
	var count int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// columnExists checks whether a column exists in the given table.
func columnExists(ctx context.Context, db *bun.DB, table, column string) bool {
	var count int
	if err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM pragma_table_info(%q) WHERE name=?", table), column,
	).Scan(&count); err != nil {
		return false
	}
	return count > 0
}
