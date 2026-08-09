package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupSpaceRepo(t *testing.T) (repositories.SpaceRepository, *bun.DB, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	return repositories.NewSpaceRepository(dbc), dbc, db.NewTransactor(dbc), cleanup
}

func newTestSpace(questID string) *models.Space {
	return &models.Space{
		QuestID:  questID,
		Slug:     gofakeit.LetterN(8),
		Name:     gofakeit.Name(),
		Kind:     game.SpaceKindPoint,
		Geometry: game.NewPointGeometry(gofakeit.Latitude(), gofakeit.Longitude(), 25),
		Payload:  gofakeit.LetterN(5),
	}
}

func TestSpaceRepository_CreateAssignsID(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	space := newTestSpace(parents.QuestID)

	require.NoError(t, repo.Create(context.Background(), space))
	assert.NotEmpty(t, space.ID, "Create should assign an ID")
}

func TestSpaceRepository_CreateKeepsSuppliedID(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	space := newTestSpace(parents.QuestID)
	space.ID = gofakeit.UUID()
	wantID := space.ID

	require.NoError(t, repo.Create(context.Background(), space))
	assert.Equal(t, wantID, space.ID)
}

// Geometry must survive the trip through the TEXT column unchanged.
func TestSpaceRepository_GeometryRoundTrip(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	space.Kind = game.SpaceKindArea
	space.Geometry = &game.SpaceGeometry{
		Type: game.GeometryPolygon,
		Rings: []game.Ring{{
			game.NewPosition(-45.87, 170.50),
			game.NewPosition(-45.88, 170.51),
			game.NewPosition(-45.89, 170.49),
			game.NewPosition(-45.87, 170.50),
		}},
	}
	require.NoError(t, repo.Create(ctx, space))

	got, err := repo.GetByID(ctx, space.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Geometry)
	assert.Equal(t, space.Geometry.Rings, got.Geometry.Rings)
	assert.Equal(t, game.SpaceKindArea, got.Kind)
}

func TestSpaceRepository_ObjectSpaceHasNoGeometry(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	space.Kind = game.SpaceKindObject
	space.Geometry = nil
	space.Mobile = true
	require.NoError(t, repo.Create(ctx, space))

	got, err := repo.GetByID(ctx, space.ID)
	require.NoError(t, err)
	assert.True(t, got.Geometry.IsZero(), "an object carries no fixed geometry")
	assert.True(t, got.Mobile)
	assert.False(t, got.HasCoordinates())
}

func TestSpaceRepository_GetByQuestAndSlug(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	space.Slug = "totara-grove"
	require.NoError(t, repo.Create(ctx, space))

	got, err := repo.GetByQuestAndSlug(ctx, parents.QuestID, "totara-grove")
	require.NoError(t, err)
	assert.Equal(t, space.ID, got.ID)

	_, err = repo.GetByQuestAndSlug(ctx, parents.QuestID, "no-such-slug")
	assert.Error(t, err)
}

func TestSpaceRepository_SlugUniquePerQuest(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	first := newTestSpace(parents.QuestID)
	first.Slug = "duplicate-slug"
	require.NoError(t, repo.Create(ctx, first))

	second := newTestSpace(parents.QuestID)
	second.Slug = "duplicate-slug"
	assert.Error(t, repo.Create(ctx, second), "slug must be unique within a quest")
}

func TestSpaceRepository_GetByPayloadIgnoresCase(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	space.Payload = "ABCDE"
	require.NoError(t, repo.Create(ctx, space))

	got, err := repo.GetByPayload(ctx, "  abcde ")
	require.NoError(t, err)
	assert.Equal(t, space.ID, got.ID)
}

func TestSpaceRepository_FindByQuestOrdersByName(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	for _, name := range []string{"Zebra House", "Aviary", "Meerkats"} {
		space := newTestSpace(parents.QuestID)
		space.Name = name
		require.NoError(t, repo.Create(ctx, space))
	}

	spaces, err := repo.FindByQuest(ctx, parents.QuestID)
	require.NoError(t, err)
	require.Len(t, spaces, 3)
	assert.Equal(t, []string{"Aviary", "Meerkats", "Zebra House"},
		[]string{spaces[0].Name, spaces[1].Name, spaces[2].Name})
}

func TestSpaceRepository_FindByQuestScopesToQuest(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	mine := createTestParents(t, dbc)
	theirs := createTestParents(t, dbc)

	require.NoError(t, repo.Create(ctx, newTestSpace(mine.QuestID)))
	require.NoError(t, repo.Create(ctx, newTestSpace(theirs.QuestID)))

	spaces, err := repo.FindByQuest(ctx, mine.QuestID)
	require.NoError(t, err)
	require.Len(t, spaces, 1)
	assert.Equal(t, mine.QuestID, spaces[0].QuestID)
}

func TestSpaceRepository_Update(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	require.NoError(t, repo.Create(ctx, space))

	space.Name = "Renamed Space"
	space.Kind = game.SpaceKindObject
	space.Mobile = true
	space.Geometry = nil
	require.NoError(t, repo.Update(ctx, space))

	got, err := repo.GetByID(ctx, space.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed Space", got.Name)
	assert.Equal(t, game.SpaceKindObject, got.Kind)
	assert.True(t, got.Mobile)
	assert.True(t, got.Geometry.IsZero(), "clearing geometry should persist")
}

func TestSpaceRepository_UpdateRequiresID(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	assert.Error(t, repo.Update(context.Background(), newTestSpace(parents.QuestID)))
}

func TestSpaceRepository_Delete(t *testing.T) {
	repo, dbc, _, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	require.NoError(t, repo.Create(ctx, space))
	require.NoError(t, repo.Delete(ctx, space.ID))

	_, err := repo.GetByID(ctx, space.ID)
	assert.Error(t, err)
}

func TestSpaceRepository_DeleteByQuest(t *testing.T) {
	repo, dbc, transactor, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	mine := createTestParents(t, dbc)
	theirs := createTestParents(t, dbc)

	require.NoError(t, repo.Create(ctx, newTestSpace(mine.QuestID)))
	require.NoError(t, repo.Create(ctx, newTestSpace(mine.QuestID)))
	require.NoError(t, repo.Create(ctx, newTestSpace(theirs.QuestID)))

	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, repo.DeleteByQuest(ctx, tx, mine.QuestID))
	require.NoError(t, tx.Commit())

	gone, err := repo.FindByQuest(ctx, mine.QuestID)
	require.NoError(t, err)
	assert.Empty(t, gone)

	kept, err := repo.FindByQuest(ctx, theirs.QuestID)
	require.NoError(t, err)
	assert.Len(t, kept, 1, "other quests' spaces should survive")
}

func TestSpaceRepository_CreateTx(t *testing.T) {
	repo, dbc, transactor, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)

	space := newTestSpace(parents.QuestID)
	require.NoError(t, repo.CreateTx(ctx, tx, space))
	require.NoError(t, tx.Commit())

	got, err := repo.GetByID(ctx, space.ID)
	require.NoError(t, err)
	assert.Equal(t, space.Name, got.Name)
}

func TestSpaceRepository_UpdateTx(t *testing.T) {
	repo, dbc, transactor, cleanup := setupSpaceRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space := newTestSpace(parents.QuestID)
	require.NoError(t, repo.Create(ctx, space))

	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	space.Name = "Updated In Tx"
	require.NoError(t, repo.UpdateTx(ctx, tx, space))
	require.NoError(t, tx.Commit())

	got, err := repo.GetByID(ctx, space.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated In Tx", got.Name)
}
