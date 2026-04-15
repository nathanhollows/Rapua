package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTeamVarStateRepo(t *testing.T) (repositories.TeamVarStateRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	repo := repositories.NewTeamVarStateRepository(dbc)
	transactor := db.NewTransactor(dbc)
	return repo, transactor, cleanup
}

func TestTeamVarStateRepository_Upsert(t *testing.T) {
	repo, _, cleanup := setupTeamVarStateRepo(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()
	parents := createTestParents(t, dbc)
	teamCode := createTestTeam(t, dbc, parents.InstanceID)

	ctx := context.Background()

	// Insert new var
	err := repo.Upsert(ctx, teamCode, parents.InstanceID, "score", "10")
	require.NoError(t, err)

	got, err := repo.GetAll(ctx, teamCode, parents.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, "10", got["score"])

	// Update existing var
	err = repo.Upsert(ctx, teamCode, parents.InstanceID, "score", "20")
	require.NoError(t, err)

	got, err = repo.GetAll(ctx, teamCode, parents.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, "20", got["score"])
}

func TestTeamVarStateRepository_GetAll(t *testing.T) {
	repo, _, cleanup := setupTeamVarStateRepo(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()
	parents := createTestParents(t, dbc)
	teamCode := createTestTeam(t, dbc, parents.InstanceID)

	ctx := context.Background()

	// Empty result for unknown team
	got, err := repo.GetAll(ctx, "XXXX", parents.InstanceID)
	require.NoError(t, err)
	assert.Empty(t, got)

	// Multiple vars
	require.NoError(t, repo.Upsert(ctx, teamCode, parents.InstanceID, "a", "1"))
	require.NoError(t, repo.Upsert(ctx, teamCode, parents.InstanceID, "b", "2"))
	require.NoError(t, repo.Upsert(ctx, teamCode, parents.InstanceID, "c", "3"))

	got, err = repo.GetAll(ctx, teamCode, parents.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, got)

	// Other team's vars not returned
	teamCode2 := createTestTeam(t, dbc, parents.InstanceID)
	require.NoError(t, repo.Upsert(ctx, teamCode2, parents.InstanceID, "a", "99"))

	got, err = repo.GetAll(ctx, teamCode, parents.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, "1", got["a"], "other team vars must not bleed through")
}

func TestTeamVarStateRepository_DeleteByTeamAndInstance(t *testing.T) {
	repo, transactor, cleanup := setupTeamVarStateRepo(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()
	parents := createTestParents(t, dbc)
	teamCode := createTestTeam(t, dbc, parents.InstanceID)

	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, teamCode, parents.InstanceID, "x", "1"))
	require.NoError(t, repo.Upsert(ctx, teamCode, parents.InstanceID, "y", "2"))

	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)

	err = repo.DeleteByTeamAndInstance(ctx, tx, teamCode, parents.InstanceID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	got, err := repo.GetAll(ctx, teamCode, parents.InstanceID)
	require.NoError(t, err)
	assert.Empty(t, got)
}
