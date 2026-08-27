package services_test

import (
	"context"
	"database/sql"
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
	repositories.ObjectiveRepository,
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
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	svc := services.NewImportService(
		slog.Default(), transactor, instanceRepo, instanceSettingsRepo, objectiveRepo, blockRepo,
	)
	return svc, instanceRepo, instanceSettingsRepo, locationRepo, objectiveRepo, blockRepo, dbc, cleanup
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

// docWithObjective returns a GameDoc with a single objective: an interactive
// proof block (satisfies PROOF_CONTEXT_NO_INTERACTIVE_BLOCK) and a content
// reveal block.
func docWithObjective(gameName, slug, title string) *game.GameDoc {
	doc := minimalValidDoc(gameName)
	doc.Structure.Children = []game.ChildDoc{
		{
			Objective: &game.ObjectiveDoc{
				Slug:  slug,
				Title: title,
				Proof: game.ObjectiveContextDoc{
					Blocks: []game.BlockDoc{
						{"type": "free_text", "prompt": "What is the answer?"},
					},
					Sets: game.SetsField{"door_unlocked": "true"},
				},
				Reveal: game.ObjectiveContextDoc{
					Blocks: []game.BlockDoc{
						{"type": "text", "content": "You found it!"},
					},
				},
			},
		},
	}
	return doc
}

func TestImportService_ImportCreate_InvalidDoc(t *testing.T) {
	svc, _, _, _, _, _, _, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Wrong version
	doc := &game.GameDoc{Rapua: "v1", Name: "Bad"}
	_, err := svc.ImportCreate(ctx, userID, doc)
	assert.Error(t, err)
}

func TestImportService_ImportCreate_MinimalDoc(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupImportService(t)
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

func TestImportService_ImportCreate_WithObjectiveAndBlocks(t *testing.T) {
	svc, _, _, _, objectiveRepo, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := docWithObjective("Hunt", "find-the-key", "Find the key")
	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Created.Objectives)
	assert.Equal(t, 2, result.Created.Blocks)

	objectives, err := objectiveRepo.FindByQuestID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, objectives, 1)
	assert.Equal(t, "find-the-key", objectives[0].Slug)
	assert.Equal(t, "Find the key", objectives[0].Title)
	assert.Equal(t, "true", objectives[0].ProofSets["door_unlocked"])

	proofBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, objectives[0].ID, game.ContextObjectiveProof)
	require.NoError(t, err)
	require.Len(t, proofBlocks, 1)
	assert.Equal(t, "free_text", proofBlocks[0].GetType())

	revealBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, objectives[0].ID, game.ContextObjectiveReveal)
	require.NoError(t, err)
	require.Len(t, revealBlocks, 1)
	assert.Equal(t, "text", revealBlocks[0].GetType())
}

func TestImportService_ImportUpdate_ReconcilesObjective(t *testing.T) {
	svc, _, _, _, objectiveRepo, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := docWithObjective("Hunt", "find-the-key", "Find the key")
	createResult, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	objectivesBefore, err := objectiveRepo.FindByQuestID(ctx, createResult.QuestID)
	require.NoError(t, err)
	require.Len(t, objectivesBefore, 1)

	// Re-import the same slug with a changed title: must update, not duplicate.
	updateDoc := docWithObjective("Hunt", "find-the-key", "Find the hidden key")
	updateResult, err := svc.ImportUpdate(ctx, userID, createResult.QuestID, updateDoc)
	require.NoError(t, err)
	assert.Equal(t, 1, updateResult.Updated.Objectives)
	assert.Equal(t, 0, updateResult.Created.Objectives)

	objectivesAfter, err := objectiveRepo.FindByQuestID(ctx, createResult.QuestID)
	require.NoError(t, err)
	require.Len(t, objectivesAfter, 1)
	assert.Equal(t, objectivesBefore[0].ID, objectivesAfter[0].ID, "same objective, not a new row")
	assert.Equal(t, "Find the hidden key", objectivesAfter[0].Title)

	// Re-import with no objectives at all: the orphan must be deleted.
	orphanObjID := objectivesAfter[0].ID
	emptyDoc := minimalValidDoc("Hunt")
	finalResult, err := svc.ImportUpdate(ctx, userID, createResult.QuestID, emptyDoc)
	require.NoError(t, err)
	assert.Equal(t, 1, finalResult.Deleted.Objectives)

	objectivesFinal, err := objectiveRepo.FindByQuestID(ctx, createResult.QuestID)
	require.NoError(t, err)
	assert.Empty(t, objectivesFinal)

	remainingBlocks, err := blockRepo.FindByOwnerID(ctx, orphanObjID)
	require.NoError(t, err)
	assert.Empty(t, remainingBlocks, "the orphan's blocks must not be left behind — owner_id has no FK to cascade them")
}

func TestImportService_ImportCreate_GameStructurePreserved(t *testing.T) {
	svc, instanceRepo, _, _, _, _, dbc, cleanup := setupImportService(t)
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
	svc, instanceRepo, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
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
						Objective: &game.ObjectiveDoc{
							Slug:  "checkpoint-a",
							Title: "Checkpoint A",
						},
					},
				},
			},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created.Groups)
	assert.Equal(t, 1, result.Created.Objectives)

	inst, err := instanceRepo.GetByID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, inst.GameStructure.SubGroups, 1)
	assert.Equal(t, "Wave 1", inst.GameStructure.SubGroups[0].Name)

	objectives, err := objectiveRepo.FindByQuestID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, objectives, 1)
	assert.Equal(t, "checkpoint-a", objectives[0].Slug)
}

func TestImportService_ImportUpdate_NotOwner(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupImportService(t)
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
	svc, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupImportService(t)
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
	svc, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupImportService(t)
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

// TestImportService_ImportUpdate_LocationsNeverDeleted: Location can no
// longer appear in a doc at all, so a naive orphan-deletion pass would delete
// every location on every update. That pass was deliberately removed:
// Location/Marker rows are left alone by import, same as by the Location ->
// Objective migration. An update-import must leave existing locations and
// their blocks untouched.
func TestImportService_ImportUpdate_LocationsNeverDeleted(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Game", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	locID := insertTestLocation(t, dbc, inst.ID)

	transactor := db.NewTransactor(dbc)
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	_, err = blockRepo.CreateTx(ctx, tx, newTextBlock(locID), locID, game.ContextLocationContent)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	inst.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyOrdered,
		CompletionType: game.CompletionAll,
		LocationIDs:    []string{locID},
		SubGroups:      []models.GameStructure{},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	// The update doc can never reference the location; the spec has no
	// representation for it any more.
	doc := minimalValidDoc("Game")
	result, err := svc.ImportUpdate(ctx, userID, inst.ID, doc)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Deleted.Objectives, "nothing to delete: the doc had no objectives to begin with")

	locations, err := locationRepo.FindByInstance(ctx, inst.ID)
	require.NoError(t, err)
	require.Len(t, locations, 1, "the location must survive an update-import")
	assert.Equal(t, locID, locations[0].ID)

	remainingBlocks, err := blockRepo.FindByOwnerID(ctx, locID)
	require.NoError(t, err)
	assert.Len(t, remainingBlocks, 1, "the location's blocks must survive too")
}

func TestImportService_ImportCreate_ObjectiveWhen(t *testing.T) {
	svc, _, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	when := &game.WhenClause{AllOf: []game.Condition{{Var: "unlocked"}}}
	doc := minimalValidDoc("When Objective Game")
	doc.Structure.Children = []game.ChildDoc{
		{
			Objective: &game.ObjectiveDoc{
				Slug:  "secret",
				Title: "Secret Spot",
				When:  when,
			},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	objs, err := objectiveRepo.FindByQuestID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	require.NotNil(t, objs[0].When)
	require.Len(t, objs[0].When.AllOf, 1)
	assert.Equal(t, "unlocked", objs[0].When.AllOf[0].Var)
}

func TestImportService_ImportCreate_GroupWhen(t *testing.T) {
	svc, instanceRepo, _, _, _, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	when := &game.WhenClause{AllOf: []game.Condition{{Var: "unlocked"}}}
	doc := minimalValidDoc("Group When Game")
	doc.Structure.Children = []game.ChildDoc{
		{
			Group: &game.GroupDoc{
				Name:       "Hidden Group",
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
	assert.Equal(t, "unlocked", subGroup.When.AllOf[0].Var)
}

func TestImportService_ImportUpdate_ObjectiveWhenUpdated(t *testing.T) {
	svc, instanceRepo, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	// Set up existing instance with an objective (no when clause).
	createDoc := docWithObjective("When Update Game", "spot", "The Spot")
	createResult, err := svc.ImportCreate(ctx, userID, createDoc)
	require.NoError(t, err)

	objs, err := objectiveRepo.FindByQuestID(ctx, createResult.QuestID)
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Nil(t, objs[0].When)

	// Load instance for update
	inst, err := instanceRepo.GetByID(ctx, createResult.QuestID)
	require.NoError(t, err)

	// Update doc: add when clause to same objective (matched by slug).
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "gate"}}}
	updateDoc := docWithObjective("When Update Game", "spot", "The Spot")
	updateDoc.Structure.Children[0].Objective.ID = objs[0].ID
	updateDoc.Structure.Children[0].Objective.When = when

	_, err = svc.ImportUpdate(ctx, userID, inst.ID, updateDoc)
	require.NoError(t, err)

	updatedObjs, err := objectiveRepo.FindByQuestID(ctx, inst.ID)
	require.NoError(t, err)
	require.Len(t, updatedObjs, 1)
	require.NotNil(t, updatedObjs[0].When)
	require.Len(t, updatedObjs[0].When.AllOf, 1)
	assert.Equal(t, "gate", updatedObjs[0].When.AllOf[0].Var)
}
