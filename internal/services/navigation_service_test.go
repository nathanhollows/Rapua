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
	teamRepo := repositories.NewRunRepository(dbc)
	instanceRepo := repositories.NewQuestRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	gameStructureService := services.NewGameStructureService(objectiveRepo, instanceRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)

	varStateRepo := repositories.NewRunVarStateRepository(dbc)
	navigationService := services.NewNavigationService(
		objectiveRepo,
		objectiveContextCompletionRepo,
		teamRepo,
		varStateRepo,
		gameStructureService,
		blockService,
		newTLogger(t),
	)

	return navigationService, teamRepo, instanceRepo, dbc, cleanup
}

// insertTestObjective inserts an objective row directly (no CreateTx-free repo
// method exists), mirroring the other insertTestX helpers in this package.
func insertTestObjective(t *testing.T, dbc *bun.DB, questID, title, slug string) *models.Objective {
	t.Helper()
	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: questID,
		Title:   title,
		Slug:    slug,
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)
	return objective
}

func createTestObjectiveGameStructure() models.GameStructure {
	return models.GameStructure{
		ID:     gofakeit.UUID(),
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             gofakeit.UUID(),
				Name:           "Group 1",
				Color:          "blue",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				ObjectiveIDs:   []string{},
			},
			{
				ID:              gofakeit.UUID(),
				Name:            "Group 2",
				Color:           "green",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false,
				Routing:         models.RouteStrategyFreeRoam,
				ObjectiveIDs:    []string{},
			},
		},
	}
}

func TestNavigationService_GetPlayerObjectiveView_FirstGroup(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := createTestObjectiveGameStructure()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	obj1 := insertTestObjective(t, dbc, instance.ID, "Objective 1", "objective-1")
	obj2 := insertTestObjective(t, dbc, instance.ID, "Objective 2", "objective-2")

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{obj1.ID, obj2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, teamPtr)

	require.NoError(t, err)
	require.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID)
	assert.False(t, view.CanAdvanceEarly)
	assert.Len(t, view.NextObjectives, 2)
}

func TestNavigationService_GetPlayerObjectiveView_OrderPreservedAfterHydration(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := createTestObjectiveGameStructure()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Insert in reverse-of-structure order so a DB-order (rather than
	// structure-order) hydration would be caught by the assertion below.
	objC := insertTestObjective(t, dbc, instance.ID, "Objective C", "objective-c")
	objB := insertTestObjective(t, dbc, instance.ID, "Objective B", "objective-b")
	objA := insertTestObjective(t, dbc, instance.ID, "Objective A", "objective-a")

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{objA.ID, objB.ID, objC.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, teamPtr)

	require.NoError(t, err)
	require.Len(t, view.NextObjectives, 3)
	assert.Equal(t, []string{objA.ID, objB.ID, objC.ID}, []string{
		view.NextObjectives[0].ID, view.NextObjectives[1].ID, view.NextObjectives[2].ID,
	}, "hydrated objectives must follow the routing-computed order, not FindByIDs' DB order")
}

func TestNavigationService_GetPlayerObjectiveView_CanAdvanceEarly(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := createTestObjectiveGameStructure()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	g1obj1 := insertTestObjective(t, dbc, instance.ID, "G1 Obj 1", "g1-obj-1")
	g1obj2 := insertTestObjective(t, dbc, instance.ID, "G1 Obj 2", "g1-obj-2")
	g2obj1 := insertTestObjective(t, dbc, instance.ID, "G2 Obj 1", "g2-obj-1")
	g2obj2 := insertTestObjective(t, dbc, instance.ID, "G2 Obj 2", "g2-obj-2")
	g2obj3 := insertTestObjective(t, dbc, instance.ID, "G2 Obj 3", "g2-obj-3")

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{g1obj1.ID, g1obj2.ID}
	instance.GameStructure.SubGroups[1].ObjectiveIDs = []string{g2obj1.ID, g2obj2.ID, g2obj3.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	// group1 fully complete, group2 minimum met (2 of 3).
	for _, id := range []string{g1obj1.ID, g1obj2.ID, g2obj1.ID, g2obj2.ID} {
		_, err = completionRepo.Insert(ctx, team.Code, id, game.ContextObjectiveReveal)
		require.NoError(t, err)
	}

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, teamPtr)

	require.NoError(t, err)
	require.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[1].ID, view.CurrentGroup.ID, "should be in group 2")
	assert.True(t, view.CanAdvanceEarly, "minimum met (2/3), AutoAdvance=false, not 100%")
	assert.Len(t, view.NextObjectives, 1, "only one objective left in group 2")
}

func TestNavigationService_GetPlayerObjectiveView_AllObjectivesVisited(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := models.GameStructure{
		ID:     gofakeit.UUID(),
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             gofakeit.UUID(),
				Name:           "Only Group",
				Color:          "blue",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				ObjectiveIDs:   []string{},
			},
		},
	}

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	obj1 := insertTestObjective(t, dbc, instance.ID, "Objective 1", "objective-1")
	obj2 := insertTestObjective(t, dbc, instance.ID, "Objective 2", "objective-2")

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{obj1.ID, obj2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	for _, id := range []string{obj1.ID, obj2.ID} {
		_, err = completionRepo.Insert(ctx, team.Code, id, game.ContextObjectiveReveal)
		require.NoError(t, err)
	}

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, teamPtr)

	require.Error(t, err)
	require.ErrorIs(t, err, services.ErrAllObjectivesVisited)
	assert.Nil(t, view)
}

func TestNavigationService_GetPreviewObjectiveView_Success(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := createTestObjectiveGameStructure()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	objective := insertTestObjective(t, dbc, instance.ID, "Preview Objective", "preview-objective")

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{objective.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	view, err := navService.GetPreviewObjectiveView(ctx, teamPtr, objective.ID)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID)
	require.Len(t, view.NextObjectives, 1)
	assert.Equal(t, objective.ID, view.NextObjectives[0].ID)
	assert.False(t, view.CanAdvanceEarly)
}

func TestNavigationService_GetPreviewObjectiveView_ObjectiveNotFound(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := createTestObjectiveGameStructure()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	_, err = navService.GetPreviewObjectiveView(ctx, teamPtr, "non-existent-id")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "objective not found in game structure")
}

// Regression test: CurrentGroup/CanAdvanceEarly used to be computed against the
// when-filtered structure while NextObjectives was derived from the unfiltered
// one. When a when-clause hid every objective in the current group, CurrentGroup
// would advance to the next group while NextObjectives stayed computed against
// the old (now-satisfied-looking) group, filtering everything out: the player
// saw "nothing to show" and got bounced to /complete despite real objectives
// pending in the group CurrentGroup already reported as current.
func TestNavigationService_GetPlayerObjectiveView_WhenFilteringConsistentAcrossCurrentGroupAndNextObjectives(t *testing.T) {
	navService, teamRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	gameStructure := createTestObjectiveGameStructure()

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settings := &models.QuestSettings{QuestID: instance.ID}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// objA is the only objective in group 1, and is permanently hidden (a var
	// that's never set can never be truthy).
	objA := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: instance.ID,
		Title:   "Hidden objective",
		Slug:    "hidden-objective",
		When:    &game.WhenClause{AllOf: []game.Condition{{Var: "never_set"}}},
	}
	_, err = dbc.NewInsert().Model(objA).Exec(ctx)
	require.NoError(t, err)

	// objB sits in group 2, always visible.
	objB := insertTestObjective(t, dbc, instance.ID, "Visible objective", "visible-objective")

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{objA.ID}
	instance.GameStructure.SubGroups[1].ObjectiveIDs = []string{objB.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	view, err := navService.GetPlayerObjectiveView(ctx, teamPtr)

	require.NoError(t, err)
	require.NotNil(t, view.CurrentGroup)
	assert.Equal(
		t, instance.GameStructure.SubGroups[1].ID, view.CurrentGroup.ID,
		"group 1 auto-satisfies once its only objective is filtered out, advancing to group 2",
	)
	require.Len(t, view.NextObjectives, 1, "objB must be surfaced, not silently dropped")
	assert.Equal(t, objB.ID, view.NextObjectives[0].ID)
}
