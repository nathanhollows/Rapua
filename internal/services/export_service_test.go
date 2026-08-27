package services_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupExportService(t *testing.T) (
	*services.ExportService,
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

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	svc := services.NewExportService(instanceRepo, instanceSettingsRepo, locationRepo, objectiveRepo, blockRepo)
	return svc, instanceRepo, instanceSettingsRepo, locationRepo, objectiveRepo, blockRepo, dbc, cleanup
}

// newTextBlock creates a MarkdownBlock for use in tests.
func newTextBlock(ownerID string) blocks.Block {
	return blocks.NewMarkdownBlock(blocks.BaseBlock{
		OwnerID: ownerID,
		Type:    "text",
		Order:   0,
	})
}

func TestExportService_ExportInstance_NotFound(t *testing.T) {
	svc, _, _, _, _, _, _, cleanup := setupExportService(t)
	defer cleanup()

	_, err := svc.ExportInstance(context.Background(), "does-not-exist")
	assert.Error(t, err)
}

func TestExportService_ExportInstance_MinimalInstance(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{
		Name:   "My Game",
		UserID: userID,
		GameStructure: models.GameStructure{
			ID:             gofakeit.UUID(),
			IsRoot:         true,
			Routing:        game.RouteStrategyOrdered,
			CompletionType: game.CompletionAll,
			LocationIDs:    []string{},
			SubGroups:      []models.GameStructure{},
		},
	}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{
		QuestID:         inst.ID,
		EnablePoints:    true,
		ShowLeaderboard: true,
	}))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	assert.Equal(t, "v8", doc.Rapua)
	assert.Equal(t, inst.ID, doc.ID)
	assert.Equal(t, "My Game", doc.Name)
	assert.True(t, doc.Settings.EnablePoints)
	assert.True(t, doc.Settings.ShowLeaderboard)
	assert.Equal(t, game.RouteStrategyOrdered, doc.Structure.Routing)
	assert.Equal(t, game.CompletionAll, doc.Structure.Completion)
	assert.Empty(t, doc.Structure.Children)
	assert.Empty(t, doc.Start)
	assert.Empty(t, doc.Finish)
}

func TestExportService_ExportInstance_WithLocationsAndBlocks(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, blockRepo, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	markerCode := gofakeit.LetterN(5)
	insertTestMarker(t, dbc, markerCode)

	inst := &models.Quest{Name: "Export Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	loc := &models.Location{
		QuestID:  inst.ID,
		MarkerID: markerCode,
		Name:     "Park",
		Slug:     "park",
		Points:   10,
	}
	require.NoError(t, locationRepo.Create(ctx, loc))

	transactor := db.NewTransactor(dbc)
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	createdBlock, err := blockRepo.CreateTx(ctx, tx, newTextBlock(loc.ID), loc.ID, game.ContextLocationContent)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	inst.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyFreeRoam,
		CompletionType: game.CompletionAll,
		LocationIDs:    []string{loc.ID},
		SubGroups:      []models.GameStructure{},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	child := doc.Structure.Children[0]
	require.NotNil(t, child.Location)
	assert.Equal(t, loc.ID, child.Location.ID)
	assert.Equal(t, "park", child.Location.Slug)
	assert.Equal(t, "Park", child.Location.Name)
	assert.Equal(t, 10, child.Location.Points)

	require.Len(t, child.Location.Content, 1)
	assert.Equal(t, "text", child.Location.Content[0]["type"])
	assert.Equal(t, createdBlock.GetID(), child.Location.Content[0]["id"])
}

func TestExportService_ExportInstance_WithObjectiveAndBlocks(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, blockRepo, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Objective Export Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	obj := &models.Objective{
		ID:         gofakeit.UUID(),
		QuestID:    inst.ID,
		Slug:       "find-the-key",
		Title:      "Find the key",
		ProofSets:  game.SetsField{"door_unlocked": "true"},
		RevealSets: game.SetsField{"story_seen": "true"},
	}
	_, err := dbc.NewInsert().Model(obj).Exec(ctx)
	require.NoError(t, err)

	transactor := db.NewTransactor(dbc)
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	proofBlock, err := blockRepo.CreateTx(ctx, tx, newTextBlock(obj.ID), obj.ID, game.ContextObjectiveProof)
	require.NoError(t, err)
	revealBlock, err := blockRepo.CreateTx(ctx, tx, newTextBlock(obj.ID), obj.ID, game.ContextObjectiveReveal)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	inst.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyFreeRoam,
		CompletionType: game.CompletionAll,
		ObjectiveIDs:   []string{obj.ID},
		SubGroups:      []models.GameStructure{},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	child := doc.Structure.Children[0]
	require.NotNil(t, child.Objective)
	assert.Equal(t, obj.ID, child.Objective.ID)
	assert.Equal(t, "find-the-key", child.Objective.Slug)
	assert.Equal(t, "Find the key", child.Objective.Title)

	require.Len(t, child.Objective.Proof.Blocks, 1)
	assert.Equal(t, proofBlock.GetID(), child.Objective.Proof.Blocks[0]["id"])
	assert.Equal(t, "true", child.Objective.Proof.Sets["door_unlocked"])

	require.Len(t, child.Objective.Reveal.Blocks, 1)
	assert.Equal(t, revealBlock.GetID(), child.Objective.Reveal.Blocks[0]["id"])
	assert.Equal(t, "true", child.Objective.Reveal.Sets["story_seen"])
}

func TestExportService_ExportInstance_StartFinishBlocks(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, _, blockRepo, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Start Finish Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	transactor := db.NewTransactor(dbc)
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	_, err = blockRepo.CreateTx(ctx, tx, newTextBlock(inst.ID), inst.ID, game.ContextStart)
	require.NoError(t, err)
	_, err = blockRepo.CreateTx(ctx, tx, newTextBlock(inst.ID), inst.ID, game.ContextFinish)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	inst.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyOrdered,
		CompletionType: game.CompletionAll,
		LocationIDs:    []string{},
		SubGroups:      []models.GameStructure{},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Start, 1)
	assert.Equal(t, "text", doc.Start[0]["type"])
	require.Len(t, doc.Finish, 1)
	assert.Equal(t, "text", doc.Finish[0]["type"])
}

func TestExportService_ExportInstance_GroupedStructure(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	markerCode := gofakeit.LetterN(5)
	insertTestMarker(t, dbc, markerCode)

	inst := &models.Quest{Name: "Grouped Game", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	loc := &models.Location{
		QuestID:  inst.ID,
		MarkerID: markerCode,
		Name:     "Spot",
		Slug:     "spot",
	}
	require.NoError(t, locationRepo.Create(ctx, loc))

	groupID := gofakeit.UUID()
	inst.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyOrdered,
		CompletionType: game.CompletionAll,
		LocationIDs:    []string{},
		SubGroups: []models.GameStructure{
			{
				ID:             groupID,
				Name:           "Wave 1",
				Color:          "primary",
				Routing:        game.RouteStrategyOrdered,
				CompletionType: game.CompletionAll,
				LocationIDs:    []string{loc.ID},
				SubGroups:      []models.GameStructure{},
			},
		},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	child := doc.Structure.Children[0]
	require.NotNil(t, child.Group)
	assert.Equal(t, "Wave 1", child.Group.Name)
	assert.Equal(t, groupID, child.Group.ID)
	require.Len(t, child.Group.Children, 1)
	require.NotNil(t, child.Group.Children[0].Location)
	assert.Equal(t, "spot", child.Group.Children[0].Location.Slug)
}

func TestExportService_LocationWhenRoundTrip(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	markerCode := gofakeit.LetterN(5)
	insertTestMarker(t, dbc, markerCode)

	inst := &models.Quest{Name: "When Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	when := &game.WhenClause{AllOf: []game.Condition{{Var: "gate"}}}
	loc := &models.Location{
		QuestID:  inst.ID,
		MarkerID: markerCode,
		Name:     "Gated Spot",
		Slug:     "gated",
		When:     when,
	}
	require.NoError(t, locationRepo.Create(ctx, loc))

	inst.GameStructure = models.GameStructure{
		ID:          gofakeit.UUID(),
		IsRoot:      true,
		LocationIDs: []string{loc.ID},
		SubGroups:   []models.GameStructure{},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	locDoc := doc.Structure.Children[0].Location
	require.NotNil(t, locDoc)
	require.NotNil(t, locDoc.When)
	require.Len(t, locDoc.When.AllOf, 1)
	assert.Equal(t, "gate", locDoc.When.AllOf[0].Var)
}

func TestExportService_GroupWhenRoundTrip(t *testing.T) {
	svc, instanceRepo, settingsRepo, locationRepo, _, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	markerCode := gofakeit.LetterN(5)
	insertTestMarker(t, dbc, markerCode)

	inst := &models.Quest{Name: "Group When Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	loc := &models.Location{
		QuestID:  inst.ID,
		MarkerID: markerCode,
		Name:     "Inner Spot",
		Slug:     "inner",
	}
	require.NoError(t, locationRepo.Create(ctx, loc))

	groupWhen := &game.WhenClause{AllOf: []game.Condition{{Var: "unlocked"}}}
	inst.GameStructure = models.GameStructure{
		ID:          gofakeit.UUID(),
		IsRoot:      true,
		LocationIDs: []string{},
		SubGroups: []models.GameStructure{
			{
				ID:          gofakeit.UUID(),
				Name:        "Hidden Group",
				Color:       "secondary",
				When:        groupWhen,
				LocationIDs: []string{loc.ID},
				SubGroups:   []models.GameStructure{},
			},
		},
	}
	require.NoError(t, instanceRepo.Update(ctx, inst))

	doc, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	groupDoc := doc.Structure.Children[0].Group
	require.NotNil(t, groupDoc)
	require.NotNil(t, groupDoc.When)
	require.Len(t, groupDoc.When.AllOf, 1)
	assert.Equal(t, "unlocked", groupDoc.When.AllOf[0].Var)
}
