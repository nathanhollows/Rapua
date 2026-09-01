// Package testdb holds helpers shared by the packages that test against a real
// database. It exists so those helpers live in one place rather than being
// copied into each package's test files, where copies drift apart.
package testdb

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// WipeData empties every application table, leaving the schema in place.
//
// The test database is in-memory and shared by a whole package: closing one
// connection does not discard it, so rows outlive the test that wrote them and
// collide with the next fixture that reuses an id, code or email. Deleting on
// the way out keeps each test starting from an empty database, including when a
// package runs more than once in one binary (go test -count).
//
// Tables belonging to the migration tool are left alone: dropping the record of
// which migrations have run would make the next setup try to apply them again.
func WipeData(t *testing.T, dbc *bun.DB) {
	t.Helper()
	ctx := context.Background()

	var tables []string
	err := dbc.NewRaw(
		`SELECT name FROM sqlite_master WHERE type = 'table' `+
			`AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'bun_%'`,
	).Scan(ctx, &tables)
	if err != nil {
		t.Fatalf("testdb.WipeData: listing tables: %v", err)
	}

	// Foreign key enforcement is per-connection in SQLite, so the pragma and
	// the deletes it applies to have to run on the same one. Order between
	// tables then stops mattering.
	conn, err := dbc.Conn(ctx)
	if err != nil {
		t.Fatalf("testdb.WipeData: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatalf("testdb.WipeData: disabling foreign keys: %v", err)
	}
	for _, table := range tables {
		if _, err := conn.ExecContext(ctx, `DELETE FROM "`+table+`"`); err != nil {
			t.Fatalf("testdb.WipeData: clearing %s: %v", table, err)
		}
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("testdb.WipeData: re-enabling foreign keys: %v", err)
	}
}
