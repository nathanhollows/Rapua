package players_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupObjectivesHandlerServices wires real NavigationService/RunService against
// an in-memory DB, matching this package's established "real services, no mocks" style.
func setupObjectivesHandlerServices(t *testing.T) (
	*services.NavigationService, *services.RunService, repositories.QuestRepository, *bun.DB, func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveContextCompletionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	varStateRepo := repositories.NewRunVarStateRepository(dbc)
	instanceRepo := repositories.NewQuestRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	creditRepo := repositories.NewCreditRepository(dbc)
	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)

	gameStructureService := services.NewGameStructureService(locationRepo, objectiveRepo, instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
	gameStructureService.SetRelationLoader(locationService)
	creditService := services.NewCreditService(transactor, creditRepo, runStartLogRepo, nil)

	navigationService := services.NewNavigationService(
		locationRepo,
		objectiveRepo,
		objectiveContextCompletionRepo,
		teamRepo,
		varStateRepo,
		gameStructureService,
		blockService,
		newTLogger(t),
	)

	runService := services.NewRunService(
		transactor, teamRepo, checkInRepo, creditService, blockStateRepo, locationRepo, varStateRepo,
		objectiveRepo, objectiveContextCompletionRepo,
	)

	return navigationService, runService, instanceRepo, dbc, cleanup
}

func createTestObjectiveQuest(t *testing.T, dbc *bun.DB, instanceRepo repositories.QuestRepository) (
	*models.Quest, *models.Run,
) {
	t.Helper()
	ctx := context.Background()

	gameStructure := models.GameStructure{
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
		},
	}

	userID := gofakeit.UUID()
	_, err := dbc.NewInsert().Model(&models.User{ID: userID, Name: gofakeit.Name(), Email: gofakeit.Email()}).Exec(ctx)
	require.NoError(t, err)

	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: gameStructure,
	}
	err = instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, &models.QuestSettings{QuestID: instance.ID})
	require.NoError(t, err)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: instance.ID,
		Title:   "Find the key",
		Slug:    "find-the-key",
	}
	_, err = dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	instance.GameStructure.SubGroups[0].ObjectiveIDs = []string{objective.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	teamRepo := repositories.NewRunRepository(dbc)
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// GetByCode (not the raw struct above) populates team.Quest.ID via its Quest
	// relation, required by LoadQuest's WherePK() the handlers call internally.
	loaded, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)

	return instance, loaded
}

func requestWithRun(target string, team *models.Run) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := context.WithValue(req.Context(), contextkeys.RunKey, team)
	return req.WithContext(ctx)
}

func TestPlayerHandler_Objectives_RendersNextObjectives(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()

	_, team := createTestObjectiveQuest(t, dbc, instanceRepo)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	w := httptest.NewRecorder()
	handler.Objectives(w, requestWithRun("/objectives", team))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Find the key")
}

func TestPlayerHandler_Objectives_RedirectsToCompleteWhenAllVisited(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()
	ctx := context.Background()

	instance, team := createTestObjectiveQuest(t, dbc, instanceRepo)

	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	objectiveID := instance.GameStructure.SubGroups[0].ObjectiveIDs[0]
	_, err := completionRepo.Insert(ctx, team.Code, objectiveID, game.ContextObjectiveReveal)
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	req := requestWithRun("/objectives", team)
	req.Header.Set("Hx-Request", "true")
	w := httptest.NewRecorder()
	handler.Objectives(w, req)

	assert.Equal(t, "/complete", w.Header().Get("Hx-Redirect"))
}

func TestPlayerHandler_Journal_RendersCompletedObjectives(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()
	ctx := context.Background()

	instance, team := createTestObjectiveQuest(t, dbc, instanceRepo)

	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	objectiveID := instance.GameStructure.SubGroups[0].ObjectiveIDs[0]
	_, err := completionRepo.Insert(ctx, team.Code, objectiveID, game.ContextObjectiveReveal)
	require.NoError(t, err)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	w := httptest.NewRecorder()
	handler.Journal(w, requestWithRun("/journal", team))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Find the key")
}

func TestPlayerHandler_Journal_EmptyWhenNothingCompleted(t *testing.T) {
	navigationService, runService, instanceRepo, dbc, cleanup := setupObjectivesHandlerServices(t)
	defer cleanup()

	_, team := createTestObjectiveQuest(t, dbc, instanceRepo)

	handler := players.NewTestPlayerHandler(
		players.WithNavigationService(navigationService),
		players.WithRunService(runService),
	)

	w := httptest.NewRecorder()
	handler.Journal(w, requestWithRun("/journal", team))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "Find the key")
	assert.Contains(t, w.Body.String(), "Nothing to show yet")
}
