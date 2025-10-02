package db_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/stretchr/testify/require"
)

// TestMustOpen_Sqlite3 ensures MustOpen successfully connects to an
// in-memory SQLite DB when the required environment variables are set.
func TestMustOpen_Sqlite3(t *testing.T) {
	// Set env vars for sqlite3
	t.Setenv("DB_TYPE", "sqlite3")
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")

	dbc := db.MustOpen()
	require.NotNil(t, dbc, "Expected a non-nil *bun.DB from MustOpen")
}

// TestMustOpen_MissingEnv tests what happens when env variables are missing.
// NOTE: This test is omitted because it would cause the program to exit.
// log.Fatal() produces a single line of output and then calls os.Exit(1),
// which is desirable in a production environment, but not in a test.

// TestMustOpen_UnsupportedDBType tests panics on an unsupported DB_TYPE.
func TestMustOpen_UnsupportedDBType(t *testing.T) {
	t.Setenv("DB_TYPE", "someInvalidDriver")
	t.Setenv("DB_CONNECTION", "fakeConnectionString")

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected MustOpen to panic for an unsupported DB_TYPE, but it did not.")
		}
	}()

	db.MustOpen() // Should panic here
}

// TestTransactor_BeginTx verifies that we can begin a transaction
// on a valid DB connection (in this case, an in-memory SQLite).
func TestTransactor_BeginTx(t *testing.T) {
	t.Setenv("DB_TYPE", "sqlite3")
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")

	dbc := db.MustOpen()
	require.NotNil(t, dbc, "Expected a valid *bun.DB from MustOpen")

	txr := db.NewTransactor(dbc)
	require.NotNil(t, txr, "Expected a non-nil Transactor")

	// Begin a transaction
	tx, err := txr.BeginTx(context.Background(), nil)
	require.NoError(t, err, "Expected no error from BeginTx")
	require.NotNil(t, tx, "Expected a non-nil *bun.Tx")

	// Clean up: rollback to avoid leaving an open transaction
	err = tx.Rollback()
	require.NoError(t, err, "Expected no error on Rollback")
}
