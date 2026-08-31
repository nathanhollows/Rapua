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

func TestObjectiveRepository_GetByID(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:         gofakeit.UUID(),
		QuestID:    parents.QuestID,
		Slug:       "find-the-key",
		Title:      "Find the key",
		ProofSets:  game.SetsField{"door_unlocked"},
		RevealSets: game.SetsField{"story_advanced"},
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.GetByID(context.Background(), objective.ID)
	require.NoError(t, err)
	assert.Equal(t, objective.Slug, got.Slug)
	assert.Equal(t, objective.Title, got.Title)
	assert.Equal(t, game.SetsField{"door_unlocked"}, got.ProofSets)
	assert.Equal(t, game.SetsField{"story_advanced"}, got.RevealSets)
}

func TestObjectiveRepository_GetByID_NotFound(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewObjectiveRepository(dbc)
	_, err := repo.GetByID(context.Background(), gofakeit.UUID())
	require.Error(t, err)
}

func TestObjectiveRepository_GetByQuestIDAndSlug(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.GetByQuestIDAndSlug(context.Background(), parents.QuestID, "find-the-key")
	require.NoError(t, err)
	assert.Equal(t, objective.ID, got.ID)
	assert.Equal(t, objective.Title, got.Title)
}

func TestObjectiveRepository_GetByQuestIDAndSlug_NotFound(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	_, err := repo.GetByQuestIDAndSlug(context.Background(), parents.QuestID, "does-not-exist")
	require.Error(t, err)
}

func TestObjectiveRepository_GetByQuestIDAndSlug_WrongQuest(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	otherParents := createTestParents(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	_, err = repo.GetByQuestIDAndSlug(context.Background(), otherParents.QuestID, "find-the-key")
	require.Error(t, err, "same slug under a different quest must not match")
}

func TestObjectiveRepository_FindByIDs(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	obj1 := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "one", Title: "One"}
	obj2 := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "two", Title: "Two"}
	obj3 := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "three", Title: "Three"}
	ctx := context.Background()
	_, err := dbc.NewInsert().Model(obj1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(obj2).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(obj3).Exec(ctx)
	require.NoError(t, err)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.FindByIDs(ctx, parents.QuestID, []string{obj1.ID, obj3.ID})
	require.NoError(t, err)
	require.Len(t, got, 2)
	gotIDs := []string{got[0].ID, got[1].ID}
	assert.ElementsMatch(t, []string{obj1.ID, obj3.ID}, gotIDs)
}

func TestObjectiveRepository_FindByIDs_Empty(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.FindByIDs(context.Background(), gofakeit.UUID(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestObjectiveRepository_FindByIDs_WrongQuest(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	obj := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "one", Title: "One"}
	ctx := context.Background()
	_, err := dbc.NewInsert().Model(obj).Exec(ctx)
	require.NoError(t, err)

	otherParents := createTestParents(t, dbc)
	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.FindByIDs(ctx, otherParents.QuestID, []string{obj.ID})
	require.NoError(t, err)
	assert.Empty(t, got, "an objective ID from a different quest must not match")
}
