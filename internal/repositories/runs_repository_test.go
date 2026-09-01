package repositories_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupTeamRepo(t *testing.T) (repositories.RunRepository, db.Transactor, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	teamRepository := repositories.NewRunRepository(dbc)
	return teamRepository, transactor, dbc, cleanup
}

func TestRunRepository_InsertTeam(t *testing.T) {
	repo, transactor, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	// Check that teams without an ID are assigned a UUID
	sampleTeam := &models.Run{
		Code:    gofakeit.Password(false, true, false, false, false, 5),
		QuestID: parents.QuestID,
	}

	err := repo.InsertBatch(ctx, []models.Run{*sampleTeam})
	require.NoError(t, err, "expected no error when saving team")

	team, err := repo.GetByCode(ctx, sampleTeam.Code)
	require.NoError(t, err, "expected no error when finding team")
	assert.NotEmpty(t, team.ID, "expected team to have an ID")

	// Check that teams with duplicate codes are not allowed
	parents2 := createTestParents(t, dbc)
	sampleTeam = &models.Run{
		ID:      gofakeit.UUID(),
		QuestID: parents2.QuestID,
	}

	err = repo.InsertBatch(ctx, []models.Run{*sampleTeam, *sampleTeam})
	require.Error(t, err, "expected error when saving teams with duplicate codes")

	// Cleanup
	for _, team := range []models.Run{*sampleTeam} {
		tx, txErr := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, txErr, "expected no error when starting transaction")

		deleteErr := repo.Delete(ctx, tx, team.QuestID, team.Code)
		if deleteErr != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, deleteErr, "expected no error when deleting team")
		} else {
			commitErr := tx.Commit()
			require.NoError(t, commitErr, "expected no error when committing transaction")
		}
	}
}

func TestRunRepository_InsertAndUpdate(t *testing.T) {
	repo, transactor, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	sampleTeam := &models.Run{
		ID:      uuid.New().String(),
		Code:    gofakeit.Password(false, true, false, false, false, 5),
		QuestID: parents.QuestID,
	}

	// Insert team first
	err := repo.InsertBatch(ctx, []models.Run{*sampleTeam})
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

	if deleteErr := repo.Delete(ctx, tx, sampleTeam.QuestID, sampleTeam.Code); deleteErr != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, deleteErr, "expected no error when deleting team")
	} else {
		commitErr := tx.Commit()
		require.NoError(t, commitErr, "expected no error when committing transaction")
	}
}

func TestRunRepository_Delete(t *testing.T) {
	repo, transactor, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	sampleTeam := []models.Run{{
		ID:      uuid.New().String(),
		Code:    gofakeit.Password(false, true, false, false, false, 5),
		QuestID: parents.QuestID,
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
	if deleteErr := repo.Delete(ctx, tx, sampleTeam[0].QuestID, sampleTeam[0].Code); deleteErr != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, deleteErr, "expected no error when deleting team")
	} else {
		commitErr := tx.Commit()
		require.NoError(t, commitErr, "expected no error when committing transaction")
	}
}

func TestRunRepository_Reset(t *testing.T) {
	repo, transactor, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	sampleTeam := []models.Run{{
		ID:      uuid.New().String(),
		Code:    gofakeit.Password(false, true, false, false, false, 4),
		QuestID: parents.QuestID,
	}}

	// Insert team first
	err := repo.Update(ctx, &sampleTeam[0])
	require.NoError(t, err, "expected no error when saving team")

	// Now reset it
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		require.NoError(t, err, "expected no error when starting transaction")
	}
	if resetErr := repo.Reset(ctx, tx, sampleTeam[0].QuestID, []string{sampleTeam[0].Code}); resetErr != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
		}
		require.NoError(t, resetErr, "expected no error when resetting team")
	} else {
		commitErr := tx.Commit()
		require.NoError(t, commitErr, "expected no error when committing transaction")
	}
}

func TestRunRepository_FindAll(t *testing.T) {
	repo, transactor, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	sampleTeams := []models.Run{
		{
			ID:      uuid.New().String(),
			Code:    gofakeit.Password(false, true, false, false, false, 5),
			QuestID: parents.QuestID,
		},
		{
			ID:      uuid.New().String(),
			Code:    gofakeit.Password(false, true, false, false, false, 5),
			QuestID: parents.QuestID,
		},
	}

	// Insert teams first
	err := repo.InsertBatch(ctx, sampleTeams)
	require.NoError(t, err, "expected no error when saving team")

	teams, err := repo.FindAll(ctx, parents.QuestID)
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
		if deleteErr := repo.Delete(ctx, tx, parents.QuestID, team.Code); deleteErr != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, deleteErr, "expected no error when deleting team")
			break
		}
	}
	commitErr := tx.Commit()
	require.NoError(t, commitErr, "expected no error when committing transaction")
}

func TestRunRepository_InsertBatch(t *testing.T) {
	repo, transactor, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents1 := createTestParents(t, dbc)
	parents2 := createTestParents(t, dbc)

	sampleTeams := []models.Run{
		{
			ID:      uuid.New().String(),
			Code:    gofakeit.Password(false, true, false, false, false, 5),
			QuestID: parents1.QuestID,
		},
		{
			ID:      uuid.New().String(),
			Code:    gofakeit.Password(false, true, false, false, false, 5),
			QuestID: parents2.QuestID,
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
		if deleteErr := repo.Delete(ctx, tx, team.QuestID, team.Code); deleteErr != nil {
			rollbackErr := tx.Rollback()
			require.NoError(t, rollbackErr, "expected no error when rolling back transaction")
			require.NoError(t, deleteErr, "expected no error when deleting team")
			break
		}
	}
	commitErr := tx.Commit()
	require.NoError(t, commitErr, "expected no error when committing transaction")
}

func TestRunRepository_InsertBatch_UniqueConstraintError(t *testing.T) {
	repo, _, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	sampleTeams := []models.Run{
		{Code: strings.ToUpper(gofakeit.LetterN(6)), QuestID: parents.QuestID},
		{Code: strings.ToUpper(gofakeit.LetterN(6)), QuestID: parents.QuestID},
	}
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
