package services_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupImportService(t *testing.T) (
	*services.ImportService,
	repositories.QuestRepository,
	repositories.QuestSettingsRepository,
	repositories.LocationRepository,
	repositories.BlockRepository,
	*bun.DB,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)

	svc := services.NewImportService(
		slog.Default(), transactor, instanceRepo, instanceSettingsRepo, locationRepo, blockRepo, markerRepo,
	)
	return svc, instanceRepo, instanceSettingsRepo, locationRepo, blockRepo, dbc, cleanup
}

// minimalValidDoc returns a valid GameDoc with no locations and no blocks.
func minimalValidDoc(name string) *game.GameDoc {
	return &game.GameDoc{
		Rapua: "v8",
		Name:  name,
		Settings: game.SettingsDoc{
			EnablePoints: true,
		},
		Start:  []game.BlockDoc{},
		Finish: []game.BlockDoc{},
		Structure: game.StructureDoc{
			Routing:    game.RouteStrategyOrdered,
			Completion: game.CompletionAll,
			Children:   []game.ChildDoc{},
		},
	}
}

// docWithLocation returns a GameDoc with a single location containing a text content block.
func docWithLocation(gameName, slug, locName string) *game.GameDoc {
	doc := minimalValidDoc(gameName)
	doc.Structure.Children = []game.ChildDoc{
		{
			Location: &game.LocationDoc{
				Slug:   slug,
				Name:   locName,
				Points: 5,
				Content: []game.BlockDoc{
					{"type": "text", "content": "Hello, world!"},
				},
			},
		},
	}
	return doc
}

func TestImportService_ImportCreate_InvalidDoc(t *testing.T) {
	svc, _, _, _, _, _, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Wrong version
	doc := &game.GameDoc{Rapua: "v1", Name: "Bad"}
	_, err := svc.ImportCreate(ctx, userID, doc)
	assert.Error(t, err)
}

func TestImportService_ImportCreate_MinimalDoc(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := minimalValidDoc("Fresh Game")
	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)
	require.NotEmpty(t, result.QuestID)

	// Verify instance was created
	inst, err := instanceRepo.GetByID(ctx, result.QuestID)
	require.NoError(t, err)
	assert.Equal(t, "Fresh Game", inst.Name)
	assert.Equal(t, userID, inst.UserID)

	// Verify settings
	settings, err := settingsRepo.GetByQuestID(ctx, result.QuestID)
	require.NoError(t, err)
	assert.True(t, settings.EnablePoints)
}

func TestImportService_ImportCreate_WithLocationsAndBlocks(t *testing.T) {
	svc, _, _, locationRepo, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := docWithLocation("Hunt", "park", "City Park")
	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Created.Locations)
	assert.Equal(t, 1, result.Created.Blocks)

	locations, err := locationRepo.FindByInstance(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, locations, 1)
	assert.Equal(t, "park", locations[0].Slug)
	assert.Equal(t, "City Park", locations[0].Name)
	assert.Equal(t, 5, locations[0].Points)

	contentBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, locations[0].ID, game.ContextLocationContent)
	require.NoError(t, err)
	require.Len(t, contentBlocks, 1)
	assert.Equal(t, "text", contentBlocks[0].GetType())
}

func TestImportService_ImportCreate_GameStructurePreserved(t *testing.T) {
	svc, instanceRepo, _, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := minimalValidDoc("Structure Test")
	doc.Structure.Routing = game.RouteStrategyFreeRoam
	doc.Structure.Completion = game.CompletionAll

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	inst, err := instanceRepo.GetByID(ctx, result.QuestID)
	require.NoError(t, err)
	assert.Equal(t, game.RouteStrategyFreeRoam, inst.GameStructure.Routing)
	assert.Equal(t, game.CompletionAll, inst.GameStructure.CompletionType)
	assert.True(t, inst.GameStructure.IsRoot)
}

func TestImportService_ImportCreate_WithGroup(t *testing.T) {
	svc, instanceRepo, _, locationRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := minimalValidDoc("Grouped Hunt")
	doc.Structure.Children = []game.ChildDoc{
		{
			Group: &game.GroupDoc{
				Name:       "Wave 1",
				Color:      "primary",
				Routing:    game.RouteStrategyOrdered,
				Completion: game.CompletionAll,
				Children: []game.ChildDoc{
					{
						Location: &game.LocationDoc{
							Slug:    "checkpoint-a",
							Name:    "Checkpoint A",
							Content: []game.BlockDoc{},
						},
					},
				},
			},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created.Groups)
	assert.Equal(t, 1, result.Created.Locations)

	inst, err := instanceRepo.GetByID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, inst.GameStructure.SubGroups, 1)
	assert.Equal(t, "Wave 1", inst.GameStructure.SubGroups[0].Name)

	locations, err := locationRepo.FindByInstance(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, locations, 1)
	assert.Equal(t, "checkpoint-a", locations[0].Slug)
}

func TestImportService_ImportUpdate_NotOwner(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	ownerID := gofakeit.UUID()
	otherUserID := gofakeit.UUID()
	insertTestUser(t, dbc, ownerID)

	inst := &models.Quest{Name: "Someone's Game", UserID: ownerID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	doc := minimalValidDoc("Hijack Attempt")
	_, err := svc.ImportUpdate(ctx, otherUserID, inst.ID, doc)
	assert.Error(t, err)
}

func TestImportService_ImportUpdate_InvalidDoc(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "My Game", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	bad := &game.GameDoc{Rapua: "v99", Name: "Bad"}
	_, err := svc.ImportUpdate(ctx, userID, inst.ID, bad)
	assert.Error(t, err)
}

func TestImportService_ImportUpdate_UpdatesInstanceAndSettings(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Old Name", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{
		QuestID:      inst.ID,
		EnablePoints: false,
	}))

	doc := minimalValidDoc("New Name")
	doc.Settings.EnablePoints = true
	doc.Settings.ShowLeaderboard = true

	result, err := svc.ImportUpdate(ctx, userID, inst.ID, doc)
	require.NoError(t, err)
	assert.Equal(t, inst.ID, result.QuestID)

	updated, err := instanceRepo.GetByID(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)

	settings, err := settingsRepo.GetByQuestID(ctx, inst.ID)
	require.NoError(t, err)
	assert.True(t, settings.EnablePoints)
	assert.True(t, settings.ShowLeaderboard)
}

func TestImportService_ImportUpdate_OrphanLocationsDeleted(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	// Create instance with a location that won't be in the update doc
	inst := &models.Quest{Name: "Game", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	orphanLocID := insertTestLocation(t, dbc, inst.ID)

	// GameStructure references the orphan
	inst.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyOrdered,
		CompletionType: game.CompletionAll,
		LocationIDs:    []string{orphanLocID},
		SubGroups:      []models.GameStructure{},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	// Update doc has no locations
	doc := minimalValidDoc("Game")
	result, err := svc.ImportUpdate(ctx, userID, inst.ID, doc)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Deleted.Locations)

	locations, err := locationRepo.FindByInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Empty(t, locations)
}

func TestImportService_ImportUpdate_ReconcileLocations(t *testing.T) {
	svc, instanceRepo, _, locationRepo, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	// Create via ImportCreate so we have real IDs
	createDoc := docWithLocation("My Hunt", "base", "Base Camp")
	createResult, err := svc.ImportCreate(ctx, userID, createDoc)
	require.NoError(t, err)

	locs, err := locationRepo.FindByInstance(ctx, createResult.QuestID)
	require.NoError(t, err)
	require.Len(t, locs, 1)
	existingLocID := locs[0].ID

	// Build export doc that references existing location by ID, plus a new one
	updateDoc := minimalValidDoc("My Hunt")
	updateDoc.Structure.Children = []game.ChildDoc{
		{
			Location: &game.LocationDoc{
				ID:      existingLocID,
				Slug:    "base",
				Name:    "Base Camp Updated",
				Points:  20,
				Content: []game.BlockDoc{},
			},
		},
		{
			Location: &game.LocationDoc{
				Slug:    "new-spot",
				Name:    "New Spot",
				Content: []game.BlockDoc{},
			},
		},
	}

	inst, err := instanceRepo.GetByID(ctx, createResult.QuestID)
	require.NoError(t, err)

	result, err := svc.ImportUpdate(ctx, userID, inst.ID, updateDoc)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Updated.Locations)
	assert.Equal(t, 1, result.Created.Locations)

	updatedLocs, err := locationRepo.FindByInstance(ctx, inst.ID)
	require.NoError(t, err)
	require.Len(t, updatedLocs, 2)

	// Find updated location by ID
	var updatedLoc *models.Location
	for i := range updatedLocs {
		if updatedLocs[i].ID == existingLocID {
			updatedLoc = &updatedLocs[i]
		}
	}
	require.NotNil(t, updatedLoc)
	assert.Equal(t, "Base Camp Updated", updatedLoc.Name)
	assert.Equal(t, 20, updatedLoc.Points)

	_ = blockRepo
}

func TestImportService_ImportCreate_LocationWhen(t *testing.T) {
	svc, _, _, locationRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	when := &game.WhenClause{AllOf: []game.Condition{{Var: "unlocked"}}}
	doc := minimalValidDoc("When Location Game")
	doc.Structure.Children = []game.ChildDoc{
		{
			Location: &game.LocationDoc{
				Slug:    "secret",
				Name:    "Secret Spot",
				When:    when,
				Content: []game.BlockDoc{},
			},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	locs, err := locationRepo.FindByInstance(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, locs, 1)
	require.NotNil(t, locs[0].When)
	require.Len(t, locs[0].When.AllOf, 1)
	assert.Equal(t, "unlocked", locs[0].When.AllOf[0].Var)
}

func TestImportService_ImportCreate_GroupWhen(t *testing.T) {
	svc, instanceRepo, _, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	when := &game.WhenClause{AllOf: []game.Condition{{Var: "phase2"}}}
	doc := minimalValidDoc("Group When Game")
	doc.Structure.Children = []game.ChildDoc{
		{
			Group: &game.GroupDoc{
				Name:       "Phase 2 Group",
				Color:      "primary",
				Routing:    game.RouteStrategyOrdered,
				Completion: game.CompletionAll,
				When:       when,
				Children:   []game.ChildDoc{},
			},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	inst, err := instanceRepo.GetByID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, inst.GameStructure.SubGroups, 1)
	subGroup := inst.GameStructure.SubGroups[0]
	require.NotNil(t, subGroup.When)
	require.Len(t, subGroup.When.AllOf, 1)
	assert.Equal(t, "phase2", subGroup.When.AllOf[0].Var)
}

func TestImportService_ImportUpdate_LocationWhenUpdated(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	// Set up existing instance with a location (no when clause)
	createDoc := docWithLocation("When Update Game", "spot", "The Spot")
	createResult, err := svc.ImportCreate(ctx, userID, createDoc)
	require.NoError(t, err)

	locs, err := locationRepo.FindByInstance(ctx, createResult.QuestID)
	require.NoError(t, err)
	require.Len(t, locs, 1)
	assert.Nil(t, locs[0].When)

	// Load instance for update
	inst, err := instanceRepo.GetByID(ctx, createResult.QuestID)
	require.NoError(t, err)
	_ = settingsRepo

	// Update doc: add when clause to same location (matched by slug)
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "gate"}}}
	updateDoc := minimalValidDoc("When Update Game")
	updateDoc.Structure.Children = []game.ChildDoc{
		{
			Location: &game.LocationDoc{
				ID:      locs[0].ID,
				Slug:    "spot",
				Name:    "The Spot",
				When:    when,
				Content: []game.BlockDoc{},
			},
		},
	}

	_, err = svc.ImportUpdate(ctx, userID, inst.ID, updateDoc)
	require.NoError(t, err)

	updatedLocs, err := locationRepo.FindByInstance(ctx, inst.ID)
	require.NoError(t, err)
	require.Len(t, updatedLocs, 1)
	require.NotNil(t, updatedLocs[0].When)
	require.Len(t, updatedLocs[0].When.AllOf, 1)
	assert.Equal(t, "gate", updatedLocs[0].When.AllOf[0].Var)
}
