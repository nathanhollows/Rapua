package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTeamRepo(t *testing.T) (repositories.TeamRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	teamRepository := repositories.NewTeamRepository(dbc)
	return teamRepository, transactor, cleanup
}

func TestTeamRepository_InsertTeam(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	// Check that teams without an ID are assigned a UUID
	sampleTeam := &models.Team{
		Code:       gofakeit.Password(false, true, false, false, false, 5),
		InstanceID: gofakeit.UUID(),
	}

	err := repo.InsertBatch(ctx, []models.Team{*sampleTeam})
	require.NoError(t, err, "expected no error when saving team")

	team, err := repo.GetByCode(ctx, sampleTeam.Code)
	require.NoError(t, err, "expected no error when finding team")
	assert.NotEmpty(t, team.ID, "expected team to have an ID")

	// Check that teams with duplicate codes are not allowed
	sampleTeam = &models.Team{
		ID:         gofakeit.UUID(),
		InstanceID: gofakeit.UUID(),
	}

	err = repo.InsertBatch(ctx, []models.Team{*sampleTeam, *sampleTeam})
	require.Error(t, err, "expected error when saving teams with duplicate codes")

	// Cleanup
	for _, team := range []models.Team{*sampleTeam} {
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err, "expected no error when starting transaction")

		err = repo.Delete(ctx, tx, team.InstanceID, team.Code)
		if err != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, err, "expected no error when deleting team")
		} else {
			err := tx.Commit()
			require.NoError(t, err, "expected no error when committing transaction")
		}
	}
}

func TestTeamRepository_InsertAndUpdate(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	sampleTeam := &models.Team{
		ID:         uuid.New().String(),
		Code:       gofakeit.Password(false, true, false, false, false, 5),
		InstanceID: gofakeit.UUID(),
	}

	// Insert team first
	err := repo.InsertBatch(ctx, []models.Team{*sampleTeam})
	require.NoError(t, err, "expected no error when saving team")

	// Check that the team was saved
	team, err := repo.GetByCode(ctx, sampleTeam.Code)
	require.NoError(t, err, "expected no error when finding team")

	// Update the team
	err = repo.Update(ctx, team)
	require.NoError(t, err, "expected no error when updating team")

	// Cleanup
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}

	if err := repo.Delete(ctx, tx, sampleTeam.InstanceID, sampleTeam.Code); err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when deleting team")
	} else {
		err := tx.Commit()
		require.NoError(t, err, "expected no error when committing transaction")
	}
}

func TestTeamRepository_Delete(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	sampleTeam := []models.Team{{
		ID:         uuid.New().String(),
		Code:       gofakeit.Password(false, true, false, false, false, 5),
		InstanceID: gofakeit.UUID(),
	}}

	// Insert team first
	err := repo.Update(ctx, &sampleTeam[0])
	require.NoError(t, err, "expected no error when saving team")

	// Now delete it
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}
	if err := repo.Delete(ctx, tx, sampleTeam[0].InstanceID, sampleTeam[0].Code); err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when deleting team")
	} else {
		err := tx.Commit()
		require.NoError(t, err, "expected no error when committing transaction")
	}
}

func TestTeamRepository_Reset(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	sampleTeam := []models.Team{{
		ID:         uuid.New().String(),
		Code:       gofakeit.Password(false, true, false, false, false, 4),
		InstanceID: gofakeit.UUID(),
	}}

	// Insert team first
	err := repo.Update(ctx, &sampleTeam[0])
	require.NoError(t, err, "expected no error when saving team")

	// Now delete it
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}
	if err := repo.Reset(ctx, tx, sampleTeam[0].InstanceID, []string{sampleTeam[0].Code}); err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		}
		require.NoError(t, err, "expected no error when resetting team")
	} else {
		err := tx.Commit()
		require.NoError(t, err, "expected no error when committing transaction")
	}
}

func TestTeamRepository_FindAll(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	instanceID := gofakeit.UUID()
	sampleTeams := []models.Team{
		{
			ID:         uuid.New().String(),
			Code:       gofakeit.Password(false, true, false, false, false, 5),
			InstanceID: instanceID,
		},
		{
			ID:         uuid.New().String(),
			Code:       gofakeit.Password(false, true, false, false, false, 5),
			InstanceID: instanceID,
		},
	}

	// Insert teams first
	err := repo.InsertBatch(ctx, sampleTeams)
	require.NoError(t, err, "expected no error when saving team")

	teams, err := repo.FindAll(ctx, instanceID)
	require.NoError(t, err, "expected no error when finding all teams")
	assert.Len(t, teams, len(sampleTeams), "expected correct number of teams to be found")

	// Cleanup
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}
	for _, team := range teams {
		if err := repo.Delete(ctx, tx, instanceID, team.Code); err != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, err, "expected no error when deleting team")
			break
		}
	}
	err = tx.Commit()
	require.NoError(t, err, "expected no error when committing transaction")
}

func TestTeamRepository_FindAllWithScans(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	instanceID := gofakeit.UUID()
	sampleTeams := []models.Team{
		{
			ID:         uuid.New().String(),
			Code:       gofakeit.Password(false, true, false, false, false, 5),
			InstanceID: instanceID,
		},
		{
			ID:         uuid.New().String(),
			Code:       gofakeit.Password(false, true, false, false, false, 5),
			InstanceID: instanceID,
		},
	}

	// Insert teams first

	err := repo.InsertBatch(ctx, sampleTeams)
	require.NoError(t, err, "expected no error when saving team")

	teams, err := repo.FindAllWithScans(ctx, instanceID)
	require.NoError(t, err, "expected no error when finding all teams with scans")
	assert.Len(t, teams, len(sampleTeams), "expected correct number of teams to be found")

	// Cleanup
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}
	for _, team := range teams {
		if err := repo.Delete(ctx, tx, instanceID, team.Code); err != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, err, "expected no error when deleting team")
			break
		}
	}
	err = tx.Commit()
	require.NoError(t, err, "expected no error when committing transaction")
}

func TestTeamRepository_InsertBatch(t *testing.T) {
	repo, transactor, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	sampleTeams := []models.Team{
		{
			ID:         uuid.New().String(),
			Code:       gofakeit.Password(false, true, false, false, false, 5),
			InstanceID: gofakeit.UUID(),
		},
		{
			ID:         uuid.New().String(),
			Code:       gofakeit.Password(false, true, false, false, false, 5),
			InstanceID: gofakeit.UUID(),
		},
	}
	err := repo.InsertBatch(ctx, sampleTeams)
	require.NoError(t, err, "expected no error when inserting batch of teams")

	// Check that the teams were saved
	for _, team := range sampleTeams {
		_, err = repo.GetByCode(ctx, team.Code)
		require.NoError(t, err, "expected no error when finding team")
	}

	// Cleanup
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}
	for _, team := range sampleTeams {
		if err := repo.Delete(ctx, tx, team.InstanceID, team.Code); err != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, err, "expected no error when deleting team")
			break
		}
	}
	err = tx.Commit()
	require.NoError(t, err, "expected no error when committing transaction")
}

func TestTeamRepository_InsertBatch_UniqueConstraintError(t *testing.T) {
	repo, _, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	sampleTeams := []models.Team{{Code: "team1"}, {Code: "team2"}}
	err := repo.InsertBatch(ctx, sampleTeams)
	require.NoError(t, err, "expected no error when inserting batch of teams")

	// Insert the same teams again to trigger unique constraint error
	err = repo.InsertBatch(ctx, sampleTeams)
	require.Error(t, err, "expected unique constraint error when inserting duplicate batch of teams")
	assert.Contains(
		t,
		err.Error(),
		"UNIQUE constraint",
		"expected error message to indicate unique constraint violation",
	)
}
