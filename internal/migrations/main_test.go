package migrations_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nathanhollows/Rapua/v5/db"
	"github.com/nathanhollows/Rapua/v5/internal/migrations"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/migrate"
)

func newTLogger(t *testing.T) *slog.Logger {
	handler := slog.NewTextHandler(testWriter{t}, nil)
	return slog.New(handler)
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

// TestFullMigration ensures the full suite runs up and with without error.
// This only tests the migrations, not the repository or service.
// Repository and service tests should ensure the migrations are correct.
func TestFullMigration(t *testing.T) {
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	db := db.MustOpen(newTLogger(t))
	ctx := context.Background()

	// Setup the migrator
	migrator := migrate.NewMigrator(db, migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		require.NoError(t, err)
	}

	if err := migrator.Lock(ctx); err != nil {
		require.NoError(t, err)
	}

	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			require.NoError(t, err)
		}
		db.Close()
	}()

	// Migrate up
	_, err := migrator.Migrate(ctx)
	if err != nil {
		require.NoError(t, err)
	}

	// Rollback the migrations
	_, err = migrator.Rollback(ctx)
	if err != nil {
		require.NoError(t, err)
	}
}
