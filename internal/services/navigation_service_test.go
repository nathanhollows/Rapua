package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The frontier itself is covered in the navigation package against in-memory
// trees. What these tests cover is the half that package cannot: that the facts
// reaching it are read from the right places.

func setupNavigationService(t *testing.T) (
	*services.NavigationService,
	repositories.RunRepository,
	repositories.QuestRepository,
	*bun.DB,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveContextCompletionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	sectionFinishRepo := repositories.NewSectionFinishRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	instanceRepo := repositories.NewQuestRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	varStateRepo := repositories.NewRunVarStateRepository(dbc)

	navigationService := services.NewNavigationService(
		objectiveRepo,
		objectiveContextCompletionRepo,
		sectionFinishRepo,
		blockRepo,
		teamRepo,
		varStateRepo,
		newTLogger(t),
	)

	return navigationService, teamRepo, instanceRepo, dbc, cleanup
}

// insertTestObjective inserts an objective row directly, so a test can build a
// tree without going through the checks the repository applies to new rows.
func insertTestObjective(t *testing.T, dbc *bun.DB, questID, parentID, title, slug string) *models.Objective {
	t.Helper()
	objective := &models.Objective{
		ID:       gofakeit.UUID(),
		QuestID:  questID,
		ParentID: parentID,
		Title:    title,
		Slug:     slug,
		Routing:  models.RouteStrategyFreeRoam,
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)
	return objective
}

// navQuest builds a quest with a root and returns both, plus a loaded run.
func navQuest(
	t *testing.T, dbc *bun.DB, teamRepo repositories.RunRepository, instanceRepo repositories.QuestRepository,
) (*models.Quest, *models.Objective, *models.Run) {
	t.Helper()
	ctx := context.Background()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	quest := &models.Quest{ID: gofakeit.UUID(), Name: "Nav Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, quest))
	require.NoError(t, repositories.NewQuestSettingsRepository(dbc).
		Create(ctx, &models.QuestSettings{QuestID: quest.ID}))

	root := insertTestObjective(t, dbc, quest.ID, "", "Nav Test", "root")

	run := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: quest.ID,
	}
	require.NoError(t, teamRepo.InsertBatch(ctx, []models.Run{run}))
	loaded, err := teamRepo.GetByCode(ctx, run.Code)
	require.NoError(t, err)
	require.NoError(t, teamRepo.LoadRelations(ctx, loaded))

	return quest, root, loaded
}

func availableSlugs(view *services.PlayerObjectiveView) []string {
	slugs := make([]string, len(view.Frontier.Available))
	for i, obj := range view.Frontier.Available {
		slugs[i] = obj.Slug
	}
	return slugs
}

func TestNavigationService_GetPlayerObjectiveView_ListsWhatThePlayerCanDo(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()

	quest, root, run := navQuest(t, dbc, teamRepo, instanceRepo)
	insertTestObjective(t, dbc, quest.ID, root.ID, "First", "first")
	insertTestObjective(t, dbc, quest.ID, root.ID, "Second", "second")

	view, err := navService.GetPlayerObjectiveView(context.Background(), run)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"first", "second"}, availableSlugs(view))
	assert.False(t, view.Complete)
}

// objective.<slug> is the only built-in, and a section has no completion row of
// its own: the resolver has to read the derived completed set, not the log.
func TestNavigationService_GetPlayerObjectiveView_ObjectiveSlugGateOpensOnCompletion(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	quest, root, run := navQuest(t, dbc, teamRepo, instanceRepo)
	gateway := insertTestObjective(t, dbc, quest.ID, root.ID, "The Gateway", "gateway")

	locked := &models.Objective{
		ID: gofakeit.UUID(), QuestID: quest.ID, ParentID: root.ID, Position: 1,
		Title: "The Inner Room", Slug: "inner-room",
		Depends: game.DependsField{"objective.gateway"},
	}
	_, err := dbc.NewInsert().Model(locked).Exec(ctx)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, run)
	require.NoError(t, err)
	assert.NotContains(t, availableSlugs(view), "inner-room",
		"the gate is shut before its objective completes")

	_, err = repositories.NewObjectiveContextCompletionRepository(dbc).
		Insert(ctx, run.Code, gateway.ID, game.ContextObjectiveProof)
	require.NoError(t, err)

	view, err = navService.GetPlayerObjectiveView(ctx, run)
	require.NoError(t, err)
	assert.Contains(t, availableSlugs(view), "inner-room",
		"the gate opens once its objective completes")
}

// A run is finished when its root completes, which is not the same as having
// nothing available: everything left may be waiting on something.
func TestNavigationService_GetPlayerObjectiveView_CompleteWhenTheRootCompletes(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	quest, root, run := navQuest(t, dbc, teamRepo, instanceRepo)
	only := insertTestObjective(t, dbc, quest.ID, root.ID, "Only", "only")

	_, err := repositories.NewObjectiveContextCompletionRepository(dbc).
		Insert(ctx, run.Code, only.ID, game.ContextObjectiveProof)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, run)
	require.NoError(t, err)
	assert.True(t, view.Complete)
	assert.Empty(t, view.Frontier.Available)
}

// Reaching the band's minimum only offers the button; the press is what
// completes the section, and the service reads the frontier to decide whether
// the press is allowed at all.
func TestNavigationService_FinishSection(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	quest, root, run := navQuest(t, dbc, teamRepo, instanceRepo)

	minChildren, maxChildren := 1, 2
	section := &models.Objective{
		ID: gofakeit.UUID(), QuestID: quest.ID, ParentID: root.ID,
		Title: "A section", Slug: "a-section", Routing: models.RouteStrategyFreeRoam,
		ChildrenMin: &minChildren, ChildrenMax: &maxChildren,
	}
	_, err := dbc.NewInsert().Model(section).Exec(ctx)
	require.NoError(t, err)

	first := insertTestObjective(t, dbc, quest.ID, section.ID, "First", "first")
	insertTestObjective(t, dbc, quest.ID, section.ID, "Second", "second")

	// Below the minimum, there is nothing to finish.
	_, err = navService.FinishSection(ctx, run, section.ID)
	require.ErrorIs(t, err, services.ErrSectionNotFinishable)

	_, err = repositories.NewObjectiveContextCompletionRepository(dbc).
		Insert(ctx, run.Code, first.ID, game.ContextObjectiveProof)
	require.NoError(t, err)

	inserted, err := navService.FinishSection(ctx, run, section.ID)
	require.NoError(t, err)
	assert.True(t, inserted)

	view, err := navService.GetPlayerObjectiveView(ctx, run)
	require.NoError(t, err)
	assert.NotContains(t, availableSlugs(view), "second",
		"finishing the section closes what was left inside it")
}

func TestNavigationService_GetPreviewObjectiveView(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()

	quest, root, run := navQuest(t, dbc, teamRepo, instanceRepo)
	objective := insertTestObjective(t, dbc, quest.ID, root.ID, "Previewed", "previewed")

	view, err := navService.GetPreviewObjectiveView(context.Background(), run, objective.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"previewed"}, availableSlugs(view),
		"a preview shows the one objective it was asked for")
}
