package repositories_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/nathanhollows/Rapua/db"
	"github.com/nathanhollows/Rapua/internal/migrate"
	"github.com/nathanhollows/Rapua/internal/repositories"
)

func setupLocationRepo(t *testing.T) (repositories.LocationRepository, func()) {
	t.Helper()
	os.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	os.Setenv("DB_TYPE", "sqlite3")
	db := db.MustOpen()

	// Create tables
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	migrate.CreateTables(logger, db)

	locationRepo := repositories.NewLocationRepository(db)
	return locationRepo, func() {
		db.Close()
	}
}
