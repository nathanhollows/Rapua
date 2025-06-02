package repositories_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
)

func setupCheckinRepo(t *testing.T) (repositories.CheckInRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	checkinRepository := repositories.NewCheckInRepository(dbc)
	return checkinRepository, transactor, cleanup
}

func TestCheckInRepository_DeleteByTeamCodes(t *testing.T) {
	repo, transactor, cleanup := setupCheckinRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Create some check-ins
	instanceID := gofakeit.UUID()
	var teams []models.Team
	for i := 0; i < 5; i++ {
		team := models.Team{
			Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
			InstanceID: instanceID,
		}
		teams = append(teams, team)
	}
	teamCodes := make([]string, 0, len(teams))
	for _, team := range teams {
		teamCodes = append(teamCodes, team.Code)
	}

	location := models.Location{
		ID:         gofakeit.UUID(),
		InstanceID: instanceID,
	}

	for _, team := range teams {
		checkin, err := repo.LogCheckIn(ctx, team, location, gofakeit.Bool(), gofakeit.Bool())
		assert.NoError(t, err, "expected no error when saving check-in")
		assert.NotEmpty(t, checkin.TimeIn, "expected check-in to have a time in")
	}

	// Now delete the check-ins using the team codes and instance ID
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			t.Fatalf("failed to rollback transaction: %v", rollbackErr)
		}
		assert.NoError(t, err, "expected no error when starting transaction")
	}

	err = repo.DeleteByTeamCodes(ctx, tx, instanceID, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			t.Fatalf("failed to rollback transaction: %v", rollbackErr)
		}
		assert.NoError(t, err, "expected no error when resetting team")
	} else {
		err = tx.Commit()
		assert.NoError(t, err, "expected no error when committing transaction")
	}

	// Check that the check-ins have been deleted
	for _, team := range teams {
		checkins, err := repo.FindCheckInByTeamAndLocation(ctx, team.Code, location.ID)
		assert.ErrorIs(t, err, sql.ErrNoRows, "expected no check-ins to be found")
		assert.Empty(t, checkins, "expected no check-ins to be found")
	}
}
