package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRunVarStateRepo(t *testing.T) (repositories.RunVarStateRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	repo := repositories.NewRunVarStateRepository(dbc)
	transactor := db.NewTransactor(dbc)
	return repo, transactor, cleanup
}

func TestRunVarStateRepository_Upsert(t *testing.T) {
	repo, _, cleanup := setupRunVarStateRepo(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	ctx := context.Background()

	// Insert new var
	err := repo.Upsert(ctx, runCode, parents.QuestID, "score", "10")
	require.NoError(t, err)

	got, err := repo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, "10", got["score"])

	// Update existing var
	err = repo.Upsert(ctx, runCode, parents.QuestID, "score", "20")
	require.NoError(t, err)

	got, err = repo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, "20", got["score"])
}

func TestRunVarStateRepository_GetAll(t *testing.T) {
	repo, _, cleanup := setupRunVarStateRepo(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	ctx := context.Background()

	got, err := repo.GetAll(ctx, "XXXX", parents.QuestID)
	require.NoError(t, err)
	assert.Empty(t, got)

	// Multiple vars
	require.NoError(t, repo.Upsert(ctx, runCode, parents.QuestID, "a", "1"))
	require.NoError(t, repo.Upsert(ctx, runCode, parents.QuestID, "b", "2"))
	require.NoError(t, repo.Upsert(ctx, runCode, parents.QuestID, "c", "3"))

	got, err = repo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, got)

	runCode2 := createTestTeam(t, dbc, parents.QuestID)
	require.NoError(t, repo.Upsert(ctx, runCode2, parents.QuestID, "a", "99"))

	got, err = repo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, "1", got["a"], "other team vars must not bleed through")
}

func TestRunVarStateRepository_DeleteByTeamAndInstance(t *testing.T) {
	repo, transactor, cleanup := setupRunVarStateRepo(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, runCode, parents.QuestID, "x", "1"))
	require.NoError(t, repo.Upsert(ctx, runCode, parents.QuestID, "y", "2"))

	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)

	err = repo.DeleteByTeamAndInstance(ctx, tx, runCode, parents.QuestID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	got, err := repo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Empty(t, got)
}
