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
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	svc := services.NewImportService(
		slog.Default(), transactor, instanceRepo, instanceSettingsRepo, objectiveRepo, blockRepo,
	)
	return svc, instanceRepo, instanceSettingsRepo, objectiveRepo, blockRepo, dbc, cleanup
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
		Structure: game.ObjectiveDoc{
			Slug:     "root",
			Title:    name,
			Routing:  game.RouteStrategyOrdered,
			Children: []game.ObjectiveDoc{},
		},
	}
}

// importedChildren returns the quest's objectives without the root, which every
// import writes from the document's structure node.
func importedChildren(
	t *testing.T, repo repositories.ObjectiveRepository, questID string,
) []models.Objective {
	t.Helper()
	objectives, err := repo.FindByQuestID(context.Background(), questID)
	require.NoError(t, err)

	out := make([]models.Objective, 0, len(objectives))
	for _, obj := range objectives {
		if obj.ParentID != "" {
			out = append(out, obj)
		}
	}
	return out
}

// docWithObjective returns a GameDoc with a single objective: an interactive
// proof block (satisfies PROOF_CONTEXT_NO_INTERACTIVE_BLOCK) and a content
// reveal block.
func docWithObjective(gameName, slug, title string) *game.GameDoc {
	doc := minimalValidDoc(gameName)
	doc.Structure.Children = []game.ObjectiveDoc{
		{
			Slug:  slug,
			Title: title,
			Proof: game.ObjectiveContextDoc{
				Blocks: []game.BlockDoc{
					{"type": "free_text", "prompt": "What is the answer?"},
				},
				Sets: game.SetsField{"door_unlocked"},
			},
			Reveal: game.ObjectiveContextDoc{
				Blocks: []game.BlockDoc{
					{"type": "text", "content": "You found it!"},
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

func TestImportService_ImportCreate_WithObjectiveAndBlocks(t *testing.T) {
	svc, _, _, objectiveRepo, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := docWithObjective("Hunt", "find-the-key", "Find the key")
	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Created.Objectives)
	assert.Equal(t, 2, result.Created.Blocks)

	objectives := importedChildren(t, objectiveRepo, result.QuestID)
	require.Len(t, objectives, 1)
	assert.Equal(t, "find-the-key", objectives[0].Slug)
	assert.Equal(t, "Find the key", objectives[0].Title)
	assert.Equal(t, game.SetsField{"door_unlocked"}, objectives[0].ProofSets)

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
	svc, _, _, objectiveRepo, blockRepo, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := docWithObjective("Hunt", "find-the-key", "Find the key")
	createResult, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	objectivesBefore := importedChildren(t, objectiveRepo, createResult.QuestID)
	require.Len(t, objectivesBefore, 1)

	// Re-import the same slug with a changed title: must update, not duplicate.
	updateDoc := docWithObjective("Hunt", "find-the-key", "Find the hidden key")
	updateResult, err := svc.ImportUpdate(ctx, userID, createResult.QuestID, updateDoc)
	require.NoError(t, err)
	assert.Equal(t, 2, updateResult.Updated.Objectives, "the root is reconciled alongside its child")
	assert.Equal(t, 0, updateResult.Created.Objectives)

	objectivesAfter := importedChildren(t, objectiveRepo, createResult.QuestID)
	require.Len(t, objectivesAfter, 1)
	assert.Equal(t, objectivesBefore[0].ID, objectivesAfter[0].ID, "same objective, not a new row")
	assert.Equal(t, "Find the hidden key", objectivesAfter[0].Title)

	// Re-import with no objectives at all: the orphan must be deleted.
	orphanObjID := objectivesAfter[0].ID
	emptyDoc := minimalValidDoc("Hunt")
	finalResult, err := svc.ImportUpdate(ctx, userID, createResult.QuestID, emptyDoc)
	require.NoError(t, err)
	assert.Equal(t, 1, finalResult.Deleted.Objectives)

	assert.Empty(t, importedChildren(t, objectiveRepo, createResult.QuestID))

	remainingBlocks, err := blockRepo.FindByOwnerID(ctx, orphanObjID)
	require.NoError(t, err)
	assert.Empty(t, remainingBlocks, "the orphan's blocks must not be left behind — owner_id has no FK to cascade them")
}

func TestImportService_ImportCreate_RootStructurePreserved(t *testing.T) {
	svc, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := minimalValidDoc("Structure Test")
	doc.Structure.Routing = game.RouteStrategyFreeRoam

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	root, err := objectiveRepo.FindRoot(ctx, result.QuestID)
	require.NoError(t, err)
	assert.Equal(t, game.RouteStrategyFreeRoam, root.Routing)
	assert.Empty(t, root.ParentID)
	assert.Nil(t, root.ChildrenMin, "an omitted band stays omitted")
	assert.Nil(t, root.ChildrenMax)
}

func TestImportService_ImportCreate_WithSection(t *testing.T) {
	svc, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := minimalValidDoc("Grouped Hunt")
	// A node with children is what storage keeps as a section.
	doc.Structure.Children = []game.ObjectiveDoc{
		{
			Slug:    "wave-1",
			Title:   "Wave 1",
			Color:   "primary",
			Routing: game.RouteStrategyOrdered,
			Children: []game.ObjectiveDoc{
				{Slug: "checkpoint-a", Title: "Checkpoint A"},
			},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Created.Groups, "the root and wave-1 both have children")
	assert.Equal(t, 1, result.Created.Objectives)

	objectives, err := objectiveRepo.FindTreeByQuestID(ctx, result.QuestID)
	require.NoError(t, err)
	require.Len(t, objectives, 3)

	bySlug := make(map[string]models.Objective, len(objectives))
	for _, obj := range objectives {
		bySlug[obj.Slug] = obj
	}
	section := bySlug["wave-1"]
	assert.Equal(t, "Wave 1", section.Title)
	assert.Equal(t, "primary", section.Color)
	assert.Equal(t, game.RouteStrategyOrdered, section.Routing)
	assert.Equal(t, section.ID, bySlug["checkpoint-a"].ParentID)
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

func TestImportService_ImportCreate_ObjectiveDepends(t *testing.T) {
	svc, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	doc := minimalValidDoc("Depends Objective Game")
	doc.Structure.Children = []game.ObjectiveDoc{
		{
			Slug:    "secret",
			Title:   "Secret Spot",
			Depends: game.DependsField{"unlocked"},
		},
	}

	result, err := svc.ImportCreate(ctx, userID, doc)
	require.NoError(t, err)

	objs := importedChildren(t, objectiveRepo, result.QuestID)
	require.Len(t, objs, 1)
	assert.Equal(t, game.DependsField{"unlocked"}, objs[0].Depends)
}

func TestImportService_ImportUpdate_ObjectiveDependsUpdated(t *testing.T) {
	svc, instanceRepo, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	// Set up existing instance with an objective (no depends).
	createDoc := docWithObjective("Depends Update Game", "spot", "The Spot")
	createResult, err := svc.ImportCreate(ctx, userID, createDoc)
	require.NoError(t, err)

	objs := importedChildren(t, objectiveRepo, createResult.QuestID)
	require.Len(t, objs, 1)
	assert.Nil(t, objs[0].Depends)

	// Load instance for update
	inst, err := instanceRepo.GetByID(ctx, createResult.QuestID)
	require.NoError(t, err)

	// Update doc: add depends to same objective (matched by slug).
	updateDoc := docWithObjective("Depends Update Game", "spot", "The Spot")
	updateDoc.Structure.Children[0].ID = objs[0].ID
	updateDoc.Structure.Children[0].Depends = game.DependsField{"gate"}

	_, err = svc.ImportUpdate(ctx, userID, inst.ID, updateDoc)
	require.NoError(t, err)

	updatedObjs := importedChildren(t, objectiveRepo, inst.ID)
	require.Len(t, updatedObjs, 1)
	assert.Equal(t, game.DependsField{"gate"}, updatedObjs[0].Depends)
}

// A document carrying an objective id from some other quest is a document about
// something else. Matching it to a same-slugged row here would silently move
// content the author never named.
func TestImportService_ImportUpdate_RejectsForeignObjectiveID(t *testing.T) {
	svc, _, _, objectiveRepo, _, dbc, cleanup := setupImportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	createResult, err := svc.ImportCreate(ctx, userID, docWithObjective("Hunt", "spot", "The Spot"))
	require.NoError(t, err)

	before := importedChildren(t, objectiveRepo, createResult.QuestID)
	require.Len(t, before, 1)

	updateDoc := docWithObjective("Hunt", "spot", "Renamed")
	updateDoc.Structure.Children[0].ID = gofakeit.UUID()

	_, err = svc.ImportUpdate(ctx, userID, createResult.QuestID, updateDoc)
	require.ErrorIs(t, err, services.ErrObjectiveIDNotInQuest)

	after := importedChildren(t, objectiveRepo, createResult.QuestID)
	require.Len(t, after, 1)
	assert.Equal(t, "The Spot", after[0].Title, "the rejected import changed nothing")
}
