package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func insertFinishableObjective(t *testing.T, dbc *bun.DB, questID, slug string) *models.Objective {
	t.Helper()
	obj := &models.Objective{
		ID: gofakeit.UUID(), QuestID: questID, Slug: slug, Title: slug,
	}
	_, err := dbc.NewInsert().Model(obj).Exec(context.Background())
	require.NoError(t, err)
	return obj
}

// The insert is the idempotency guard: a second press must report that it
// changed nothing, so a caller never fires the same completion twice.
func TestSectionFinishRepository_InsertIsIdempotent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)
	section := insertFinishableObjective(t, dbc, parents.QuestID, "wing")

	repo := repositories.NewSectionFinishRepository(dbc)

	inserted, err := repo.Insert(ctx, runCode, section.ID)
	require.NoError(t, err)
	assert.True(t, inserted, "the first press records the finish")

	inserted, err = repo.Insert(ctx, runCode, section.ID)
	require.NoError(t, err)
	assert.False(t, inserted, "a second press changes nothing")

	ids, err := repo.FindFinishedObjectiveIDs(ctx, runCode)
	require.NoError(t, err)
	assert.Equal(t, []string{section.ID}, ids)
}

func TestSectionFinishRepository_FindFinishedObjectiveIDs_ScopedToTheRun(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	mine := createTestTeam(t, dbc, parents.QuestID)
	theirs := createTestTeam(t, dbc, parents.QuestID)
	section := insertFinishableObjective(t, dbc, parents.QuestID, "wing")

	repo := repositories.NewSectionFinishRepository(dbc)
	_, err := repo.Insert(ctx, theirs, section.ID)
	require.NoError(t, err)

	ids, err := repo.FindFinishedObjectiveIDs(ctx, mine)
	require.NoError(t, err)
	assert.Empty(t, ids, "one run's press must not finish another run's section")
}

func TestSectionFinishRepository_FindFinishedObjectiveIDs_NoneRecorded(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	ids, err := repositories.NewSectionFinishRepository(dbc).
		FindFinishedObjectiveIDs(context.Background(), runCode)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// Rows hang off both a run and an objective, so removing either must not leave
// the log pointing at something gone.
func TestSectionFinishRepository_CascadesWithTheObjective(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)
	section := insertFinishableObjective(t, dbc, parents.QuestID, "wing")

	repo := repositories.NewSectionFinishRepository(dbc)
	_, err := repo.Insert(ctx, runCode, section.ID)
	require.NoError(t, err)

	_, err = dbc.NewDelete().Model((*models.Objective)(nil)).Where("id = ?", section.ID).Exec(ctx)
	require.NoError(t, err)

	ids, err := repo.FindFinishedObjectiveIDs(ctx, runCode)
	require.NoError(t, err)
	assert.Empty(t, ids)
}
