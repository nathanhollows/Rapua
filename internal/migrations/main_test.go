package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/internal/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/migrate"
)

// TestFullMigration ensures the full suite runs up and with without error.
// This only tests the migrations, not the repository or service.
// Repository and service tests should ensure the migrations are correct.
func TestFullMigration(t *testing.T) {
	os.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	os.Setenv("DB_TYPE", "sqlite3")
	db := db.MustOpen()
	ctx := context.Background()

	// Setup the migrator
	migrator := migrate.NewMigrator(db, migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		assert.NoError(t, err)
	}

	if err := migrator.Lock(ctx); err != nil {
		assert.NoError(t, err)
	}

	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			assert.NoError(t, err)
		}
		db.Close()
	}()

	// Migrate up
	_, err := migrator.Migrate(ctx)
	if err != nil {
		assert.NoError(t, err)
	}

	// Rollback the migrations
	_, err = migrator.Rollback(ctx)
	if err != nil {
		assert.NoError(t, err)
	}
}
