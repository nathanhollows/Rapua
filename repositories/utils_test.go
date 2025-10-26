package repositories_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nathanhollows/Rapua/v5/db"
	"github.com/nathanhollows/Rapua/v5/internal/migrations"
	"github.com/uptrace/bun"
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

func setupDB(t *testing.T) (*bun.DB, func()) {
	t.Helper()
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	db := db.MustOpen(newTLogger(t))
	ctx := context.Background()

	migrator := migrate.NewMigrator(db, migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if err := migrator.Lock(ctx); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			t.Fatal(err)
		}
	}()

	_, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return db, func() {
		db.Close()
	}
}
