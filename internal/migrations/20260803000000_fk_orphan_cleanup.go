package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// Repairs foreign key violations left by 20260329000000, which swept orphaned
// children before deleting the instances that orphaned them. The sweep order is
// fixed now, but databases that already ran it keep the bad rows.
func init() {
	Migrations.MustRegister(
		repairFKViolations,
		func(_ context.Context, _ *bun.DB) error {
			// Deleted orphans cannot be reconstructed.
			return nil
		},
	)
}

type fkViolation struct {
	table string
	rowID int64
	fkID  int
}

// repairFKViolations resolves every row reported by PRAGMA foreign_key_check,
// honouring each constraint's declared ON DELETE action. Deleting an orphan can
// orphan its own children, so it repeats until the check comes back clean.
func repairFKViolations(ctx context.Context, db *bun.DB) error {
	const maxPasses = 10

	for pass := range maxPasses {
		violations, err := findFKViolations(ctx, db)
		if err != nil {
			return err
		}
		if len(violations) == 0 {
			return nil
		}
		for _, v := range violations {
			if err := repairFKViolation(ctx, db, v); err != nil {
				return fmt.Errorf("pass %d: %w", pass, err)
			}
		}
	}

	remaining, err := findFKViolations(ctx, db)
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("foreign key violations remain after %d passes (%d rows, first: %s)",
			maxPasses, len(remaining), remaining[0].table)
	}
	return nil
}

func findFKViolations(ctx context.Context, db *bun.DB) ([]fkViolation, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, fmt.Errorf("foreign_key_check: %w", err)
	}
	defer rows.Close()

	var out []fkViolation
	for rows.Next() {
		var table, parent string
		var rowID *int64
		var fkID int
		if err := rows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			return nil, fmt.Errorf("scanning foreign_key_check: %w", err)
		}
		// NULL for WITHOUT ROWID tables, which cannot be targeted this way.
		if rowID == nil {
			continue
		}
		out = append(out, fkViolation{table: table, rowID: *rowID, fkID: fkID})
	}
	return out, rows.Err()
}

func repairFKViolation(ctx context.Context, db *bun.DB, v fkViolation) error {
	column, onDelete, err := fkColumnAction(ctx, db, v.table, v.fkID)
	if err != nil {
		return err
	}

	nullable, err := columnIsNullable(ctx, db, v.table, column)
	if err != nil {
		return err
	}

	// SET NULL on a NOT NULL column cannot be honoured, so it falls through to
	// the delete alongside CASCADE and RESTRICT.
	if strings.EqualFold(onDelete, "SET NULL") && nullable {
		sql := fmt.Sprintf("UPDATE %s SET %s = NULL WHERE rowid = ?", quoteIdent(v.table), quoteIdent(column))
		if _, err := db.ExecContext(ctx, sql, v.rowID); err != nil {
			return fmt.Errorf("blanking %s.%s rowid %d: %w", v.table, column, v.rowID, err)
		}
		return nil
	}

	sql := fmt.Sprintf("DELETE FROM %s WHERE rowid = ?", quoteIdent(v.table))
	if _, err := db.ExecContext(ctx, sql, v.rowID); err != nil {
		return fmt.Errorf("deleting orphan %s rowid %d: %w", v.table, v.rowID, err)
	}
	return nil
}

func fkColumnAction(ctx context.Context, db *bun.DB, table string, fkID int) (string, string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(table)))
	if err != nil {
		return "", "", fmt.Errorf("foreign_key_list(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, seq int
		var parent, from, onUpdate, onDel, match string
		// NULL when the constraint targets the parent's primary key.
		var to *string
		if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDel, &match); err != nil {
			return "", "", fmt.Errorf("scanning foreign_key_list(%s): %w", table, err)
		}
		if id == fkID {
			return from, onDel, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	return "", "", fmt.Errorf("no foreign key %d on table %s", fkID, table)
}

func columnIsNullable(ctx context.Context, db *bun.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(table)))
	if err != nil {
		return false, fmt.Errorf("table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt *string
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scanning table_info(%s): %w", table, err)
		}
		if name == column {
			return notNull == 0 && pk == 0, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, fmt.Errorf("no column %s on table %s", column, table)
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
