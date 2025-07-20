package repositories_test

import (
	"context"
	"os"
	"testing"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/internal/migrations"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

func setupDB(t *testing.T) (*bun.DB, func()) {
	t.Helper()
	os.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	os.Setenv("DB_TYPE", "sqlite3")
	db := db.MustOpen()
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
