package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunVariableRepository_InsertReportsOnlyNewNames(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	run := &models.Run{ID: gofakeit.UUID(), Code: gofakeit.LetterN(6), Name: "test-run", QuestID: parents.QuestID}
	_, err := dbc.NewInsert().Model(run).Exec(ctx)
	require.NoError(t, err)

	repo := repositories.NewRunVariableRepository(dbc)

	tx, err := dbc.BeginTx(ctx, nil)
	require.NoError(t, err)
	set, err := repo.InsertWithTx(ctx, &tx, run.Code, "block-a", []string{"left", "river", "left"})
	require.NoError(t, err)
	assert.Equal(t, []string{"left", "river"}, set)

	set, err = repo.InsertWithTx(ctx, &tx, run.Code, "block-b", []string{"river", "door"})
	require.NoError(t, err)
	assert.Equal(t, []string{"door"}, set, "a name already set is not set again")
	require.NoError(t, tx.Commit())

	names, err := repo.FindNamesByRunCode(ctx, run.Code)
	require.NoError(t, err)
	assert.Equal(t, []string{"door", "left", "river"}, names)

	var first models.RunVariable
	require.NoError(t, dbc.NewSelect().Model(&first).Where("run_code = ? AND name = ?", run.Code, "river").Scan(ctx))
	assert.Equal(t, "block-a", first.BlockID, "the block that set it first keeps the credit")
}

func TestRunVariableRepository_RolledBackAnswerSetsNothing(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	run := &models.Run{ID: gofakeit.UUID(), Code: gofakeit.LetterN(6), Name: "test-run", QuestID: parents.QuestID}
	_, err := dbc.NewInsert().Model(run).Exec(ctx)
	require.NoError(t, err)

	repo := repositories.NewRunVariableRepository(dbc)
	tx, err := dbc.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = repo.InsertWithTx(ctx, &tx, run.Code, "block-a", []string{"left"})
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	names, err := repo.FindNamesByRunCode(ctx, run.Code)
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestObjective_DependsRoundTrips(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "door", Title: "Door",
		Depends: game.DependsField{"key", "lamp"},
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	var got models.Objective
	require.NoError(t, dbc.NewSelect().Model(&got).Where("id = ?", objective.ID).Scan(ctx))
	assert.Equal(t, game.DependsField{"key", "lamp"}, got.Depends)

	ungated := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "hall", Title: "Hall"}
	_, err = dbc.NewInsert().Model(ungated).Exec(ctx)
	require.NoError(t, err)
	var raw *string
	require.NoError(
		t,
		dbc.NewSelect().Table("objectives").Column("depends").Where("id = ?", ungated.ID).Scan(ctx, &raw),
	)
	assert.Nil(t, raw, "no gate is stored as NULL, not an empty list")
}
