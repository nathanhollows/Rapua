package repositories_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
)

func setupCheckinRepo(t *testing.T) (repositories.CheckInRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	checkinRepository := repositories.NewCheckInRepository(dbc)
	return checkinRepository, transactor, cleanup
}
