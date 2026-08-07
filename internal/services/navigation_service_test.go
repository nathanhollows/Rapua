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
	repositories.LocationRepository,
	repositories.RunRepository,
	repositories.CheckInRepository,
	repositories.QuestRepository,
	*bun.DB,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	locationRepo := repositories.NewLocationRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	instanceRepo := repositories.NewQuestRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	gameStructureService := services.NewGameStructureService(locationRepo, instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
	gameStructureService.SetRelationLoader(locationService)

	varStateRepo := repositories.NewRunVarStateRepository(dbc)
	navigationService := services.NewNavigationService(
		locationRepo,
		teamRepo,
		varStateRepo,
		gameStructureService,
		blockService,
		newTLogger(t),
	)

	return navigationService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup
}

// createTestGameStructure creates a game structure for testing with 3 groups.
func createTestGameStructure() models.GameStructure {
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
				LocationIDs:    []string{}, // Will be filled with actual location IDs
			},
			{
				ID:              gofakeit.UUID(),
				Name:            "Group 2",
				Color:           "green",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false, // Key: can advance early
				Routing:         models.RouteStrategyFreeRoam,
				LocationIDs:     []string{}, // Will be filled
			},
			{
				ID:             gofakeit.UUID(),
				Name:           "Group 3",
				Color:          "red",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				LocationIDs:    []string{}, // Will be filled
			},
		},
	}
}

func TestNavigationService_GetPlayerNavigationView_TeamBlocked(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create FK-valid user for instance
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	// Create instance with game structure
	instance := &models.Quest{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        userID,
		GameStructure: createTestGameStructure(),
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create blocking location
	markerCode := gofakeit.LetterN(5)
	insertTestMarker(t, dbc, markerCode)
	blockingLocation := &models.Location{
		QuestID:  instance.ID,
		Name:     "Blocking Location",
		MarkerID: markerCode,
	}
	err = locationRepo.Create(ctx, blockingLocation)
	require.NoError(t, err)

	// Create team with MustCheckOut set
	team := models.Run{
		ID:           gofakeit.UUID(),
		Code:         strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:         "Test Team",
		QuestID:      instance.ID,
		MustCheckOut: blockingLocation.ID, // Use location.ID
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Load team with relations
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.True(t, view.MustCheckOut)
	assert.NotNil(t, view.BlockingLocation)
	assert.Equal(t, blockingLocation.ID, view.BlockingLocation.ID)
	assert.Empty(t, view.NextLocations)
	assert.False(t, view.CanAdvanceEarly)
}

func TestNavigationService_GetPlayerNavigationView_FirstGroup(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for group 1
	markerID1 := gofakeit.UUID()
	insertTestMarker(t, dbc, markerID1)
	loc1 := &models.Location{
		QuestID:  instance.ID,
		Name:     "Location 1",
		MarkerID: markerID1,
	}
	markerID2 := gofakeit.UUID()
	insertTestMarker(t, dbc, markerID2)
	loc2 := &models.Location{
		QuestID:  instance.ID,
		Name:     "Location 2",
		MarkerID: markerID2,
	}
	err = locationRepo.Create(ctx, loc1)
	require.NoError(t, err)
	err = locationRepo.Create(ctx, loc2)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{loc1.ID, loc2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team with no check-ins
	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID)
	assert.False(t, view.MustCheckOut)
	assert.False(t, view.CanAdvanceEarly) // Not minimum met yet
	assert.Len(t, view.NextLocations, 2)
}

func TestNavigationService_GetPlayerNavigationView_CanAdvanceEarly(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for group 1 and 2
	group1Locs := []*models.Location{}
	for range 2 {
		markerID := gofakeit.UUID()
		insertTestMarker(t, dbc, markerID)
		loc := &models.Location{
			QuestID:  instance.ID,
			Name:     gofakeit.StreetName(),
			MarkerID: markerID,
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group1Locs = append(group1Locs, loc)
	}

	group2Locs := []*models.Location{}
	for range 3 {
		markerID := gofakeit.UUID()
		insertTestMarker(t, dbc, markerID)
		loc := &models.Location{
			QuestID:  instance.ID,
			Name:     gofakeit.StreetName(),
			MarkerID: markerID,
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group2Locs = append(group2Locs, loc)
	}

	// Update game structure
	instance.GameStructure.SubGroups[0].LocationIDs = []string{group1Locs[0].ID, group1Locs[1].ID}
	instance.GameStructure.SubGroups[1].LocationIDs = []string{group2Locs[0].ID, group2Locs[1].ID, group2Locs[2].ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Create check-ins for group 1 (complete) and group 2 (minimum met: 2/3)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[1], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[1], false, false)
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[1].ID, view.CurrentGroup.ID, "should be in group 2")
	assert.True(t, view.CanAdvanceEarly, "minimum met (2/3), AutoAdvance=false, not 100%")
	assert.Len(t, view.NextLocations, 1, "only one location left in group 2")
}

func TestNavigationService_GetPlayerNavigationView_SkippedGroup(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for all groups
	group1Locs := []*models.Location{}
	for range 2 {
		markerID := gofakeit.UUID()
		insertTestMarker(t, dbc, markerID)
		loc := &models.Location{
			QuestID:  instance.ID,
			Name:     gofakeit.StreetName(),
			MarkerID: markerID,
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group1Locs = append(group1Locs, loc)
	}

	group2Locs := []*models.Location{}
	for range 3 {
		markerID := gofakeit.UUID()
		insertTestMarker(t, dbc, markerID)
		loc := &models.Location{
			QuestID:  instance.ID,
			Name:     gofakeit.StreetName(),
			MarkerID: markerID,
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group2Locs = append(group2Locs, loc)
	}

	group3MarkerID := gofakeit.UUID()
	insertTestMarker(t, dbc, group3MarkerID)
	group3Loc := &models.Location{
		QuestID:  instance.ID,
		Name:     gofakeit.StreetName(),
		MarkerID: group3MarkerID,
	}
	err = locationRepo.Create(ctx, group3Loc)
	require.NoError(t, err)

	// Update game structure
	instance.GameStructure.SubGroups[0].LocationIDs = []string{group1Locs[0].ID, group1Locs[1].ID}
	instance.GameStructure.SubGroups[1].LocationIDs = []string{group2Locs[0].ID, group2Locs[1].ID, group2Locs[2].ID}
	instance.GameStructure.SubGroups[2].LocationIDs = []string{group3Loc.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team with group2 skipped
	team := models.Run{
		ID:              gofakeit.UUID(),
		Code:            strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:            "Test Team",
		QuestID:         instance.ID,
		SkippedGroupIDs: []string{instance.GameStructure.SubGroups[1].ID}, // Skip group 2
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Create check-ins for group 1 (complete) and group 2 (minimum met but will be skipped)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[1], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[1], false, false)
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[2].ID, view.CurrentGroup.ID, "should skip to group 3")
	assert.False(t, view.CanAdvanceEarly, "group 3 has AutoAdvance=true")
	assert.Len(t, view.NextLocations, 1)
}

func TestNavigationService_GetPreviewNavigationView_Success(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create location for group 1
	markerID := gofakeit.UUID()
	insertTestMarker(t, dbc, markerID)
	location := &models.Location{
		QuestID:  instance.ID,
		Name:     "Preview Location",
		MarkerID: markerID,
	}
	err = locationRepo.Create(ctx, location)
	require.NoError(t, err)

	// Update game structure with location ID
	instance.GameStructure.SubGroups[0].LocationIDs = []string{location.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPreviewNavigationView(ctx, teamPtr, location.ID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID)
	assert.Len(t, view.NextLocations, 1)
	assert.Equal(t, location.ID, view.NextLocations[0].ID)
	assert.False(t, view.MustCheckOut)
	assert.False(t, view.CanAdvanceEarly)
}

func TestNavigationService_GetPreviewNavigationView_LocationNotFound(t *testing.T) {
	navService, _, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create team
	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute with non-existent location ID
	_, err = navService.GetPreviewNavigationView(ctx, teamPtr, "non-existent-id")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "location not found in game structure")
}

func TestNavigationService_GetPreviewNavigationView_FirstGroupDefault(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure with multiple groups
	gameStructure := createTestGameStructure()

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for all three groups
	g1MarkerID := gofakeit.UUID()
	insertTestMarker(t, dbc, g1MarkerID)
	group1Loc := &models.Location{
		QuestID:  instance.ID,
		Name:     "Group 1 Location",
		MarkerID: g1MarkerID,
	}
	err = locationRepo.Create(ctx, group1Loc)
	require.NoError(t, err)

	g2MarkerID := gofakeit.UUID()
	insertTestMarker(t, dbc, g2MarkerID)
	group2Loc := &models.Location{
		QuestID:  instance.ID,
		Name:     "Group 2 Location",
		MarkerID: g2MarkerID,
	}
	err = locationRepo.Create(ctx, group2Loc)
	require.NoError(t, err)

	g3MarkerID := gofakeit.UUID()
	insertTestMarker(t, dbc, g3MarkerID)
	group3Loc := &models.Location{
		QuestID:  instance.ID,
		Name:     "Group 3 Location",
		MarkerID: g3MarkerID,
	}
	err = locationRepo.Create(ctx, group3Loc)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{group1Loc.ID}
	instance.GameStructure.SubGroups[1].LocationIDs = []string{group2Loc.ID}
	instance.GameStructure.SubGroups[2].LocationIDs = []string{group3Loc.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute preview for first group's first location
	view, err := navService.GetPreviewNavigationView(ctx, teamPtr, group1Loc.ID)

	// Assert - should show first group (SubGroups[0]), not root or current team state
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID, "should show first group")
	assert.Equal(t, "Group 1", view.CurrentGroup.Name)
	assert.Len(t, view.NextLocations, 1)
	assert.Equal(t, group1Loc.ID, view.NextLocations[0].ID)
}

func TestNavigationService_GetPlayerNavigationView_AllLocationsVisited(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a simple game structure with one group
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
				LocationIDs:    []string{},
			},
		},
	}

	// Create instance
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

	// Create instance settings
	settings := &models.QuestSettings{
		QuestID: instance.ID,
	}
	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create two locations
	markerID1 := strings.ToUpper(gofakeit.LetterN(5))
	insertTestMarker(t, dbc, markerID1)
	loc1 := &models.Location{
		QuestID:  instance.ID,
		Name:     "Location 1",
		MarkerID: markerID1,
	}
	markerID2 := strings.ToUpper(gofakeit.LetterN(5))
	insertTestMarker(t, dbc, markerID2)
	loc2 := &models.Location{
		QuestID:  instance.ID,
		Name:     "Location 2",
		MarkerID: markerID2,
	}
	err = locationRepo.Create(ctx, loc1)
	require.NoError(t, err)
	err = locationRepo.Create(ctx, loc2)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{loc1.ID, loc2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Run{
		ID:      gofakeit.UUID(),
		Code:    strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:    "Test Team",
		QuestID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Run{team})
	require.NoError(t, err)

	// Create check-ins for ALL locations (completing the game)
	_, err = checkInRepo.LogCheckIn(ctx, team, *loc1, false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *loc2, false, false)
	require.NoError(t, err)

	// Load team with relations
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert - should return ErrAllLocationsVisited
	require.Error(t, err)
	require.ErrorIs(t, err, services.ErrAllLocationsVisited)
	assert.Nil(t, view)
}

// TestNavigationService_WhenConditionUsesVarStates is a regression test for the
// /next ↔ /complete redirect loop.
//
// Root cause: GetPlayerNavigationView evaluated `when` conditions on locations,
// but the navigation service's ensureTeamRelationsLoaded did not reload VarStates
// after they were written by a block's `sets` trigger. This caused the conditional
// location to be hidden (var not seen → when=false), while Complete's old
// GetNextLocations call (no when-filtering) still returned it, creating a loop:
//
//	/next → "conditional loc hidden, nothing left" → /complete
//	/complete → "loc exists (no filter)" → /next → ...
//
// The fix: navigation service always reloads VarStates; Complete uses
// GetPlayerNavigationView (same filtering as /next).
func TestNavigationService_WhenConditionUsesVarStates(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	varStateRepo := repositories.NewRunVarStateRepository(dbc)

	// Create a simple two-location game inside a single group.
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	instance := &models.Quest{
		ID:     gofakeit.UUID(),
		Name:   "Regression Test Game",
		UserID: userID,
		GameStructure: models.GameStructure{
			ID:     gofakeit.UUID(),
			IsRoot: true,
			SubGroups: []models.GameStructure{
				{
					ID:             gofakeit.UUID(),
					Name:           "Main",
					CompletionType: models.CompletionAll,
					AutoAdvance:    true,
					Routing:        models.RouteStrategyOrdered,
					LocationIDs:    []string{}, // filled below
				},
			},
		},
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	settingsRepo := repositories.NewQuestSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, &models.QuestSettings{QuestID: instance.ID})
	require.NoError(t, err)

	// loc1: always visible (no when clause)
	markerCode1 := gofakeit.UUID()
	insertTestMarker(t, dbc, markerCode1)
	loc1 := &models.Location{
		QuestID:  instance.ID,
		MarkerID: markerCode1,
		Name:     "Location 1",
	}
	err = locationRepo.Create(ctx, loc1)
	require.NoError(t, err)

	// loc2: only visible when var "visited_loc1" is truthy
	markerCode2 := gofakeit.UUID()
	insertTestMarker(t, dbc, markerCode2)
	loc2 := &models.Location{
		QuestID:  instance.ID,
		MarkerID: markerCode2,
		Name:     "Location 2",
		When: &game.WhenClause{
			AllOf: []game.Condition{{Var: "visited_loc1"}},
		},
	}
	err = locationRepo.Create(ctx, loc2)
	require.NoError(t, err)

	// Wire both locations into the group.
	instance.GameStructure.SubGroups[0].LocationIDs = []string{loc1.ID, loc2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	runCode := strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4))
	insertTestTeam(t, dbc, runCode, instance.ID)

	loadTeam := func() *models.Run {
		t.Helper()
		tp, e := teamRepo.GetByCode(ctx, runCode)
		require.NoError(t, e)
		require.NoError(t, teamRepo.LoadRelations(ctx, tp))
		return tp
	}

	t.Run("loc2 hidden before var is set", func(t *testing.T) {
		team := loadTeam()
		view, err := navService.GetPlayerNavigationView(ctx, team)
		require.NoError(t, err)
		// Only loc1 should be offered — loc2's when clause hides it.
		require.Len(t, view.NextLocations, 1)
		assert.Equal(t, loc1.ID, view.NextLocations[0].ID)
	})

	t.Run("loc2 visible after var is set", func(t *testing.T) {
		// Simulate the block sets trigger writing the var.
		err := varStateRepo.Upsert(ctx, runCode, instance.ID, "visited_loc1", "true")
		require.NoError(t, err)

		// Check in to loc1 so it's marked completed.
		team := loadTeam()
		_, err = checkInRepo.LogCheckIn(ctx, *team, *loc1, false, false)
		require.NoError(t, err)

		// Reload team — navigation service must pick up the new VarState.
		team = loadTeam()
		view, err := navService.GetPlayerNavigationView(ctx, team)
		require.NoError(t, err)

		// loc2 should now be visible because visited_loc1 is truthy.
		require.Len(t, view.NextLocations, 1,
			"conditional loc2 should appear once var is set (regression: VarStates were not reloaded)")
		assert.Equal(t, loc2.ID, view.NextLocations[0].ID)
	})
}
