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
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	svc := services.NewExportService(instanceRepo, instanceSettingsRepo, objectiveRepo, blockRepo)
	return svc, instanceRepo, instanceSettingsRepo, blockRepo, dbc, cleanup
}

// insertObjective writes an objective row directly, bypassing the repository so
// tests can pin IDs and positions.
func insertObjective(t *testing.T, dbc *bun.DB, obj *models.Objective) {
	t.Helper()
	_, err := dbc.NewInsert().Model(obj).Exec(context.Background())
	require.NoError(t, err)
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
	svc, _, _, _, _, cleanup := setupExportService(t)
	defer cleanup()

	_, _, err := svc.ExportInstance(context.Background(), "does-not-exist")
	assert.Error(t, err)
}

func TestExportService_ExportInstance_MinimalInstance(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "My Game", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	insertObjective(t, dbc, &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: inst.ID,
		Slug:    "root",
		Title:   "My Game",
		Routing: game.RouteStrategyOrdered,
	})
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{
		QuestID:         inst.ID,
		EnablePoints:    true,
		ShowLeaderboard: true,
	}))

	doc, _, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	assert.Equal(t, "v8", doc.Rapua)
	assert.Equal(t, inst.ID, doc.ID)
	assert.Equal(t, "My Game", doc.Name)
	assert.True(t, doc.Settings.EnablePoints)
	assert.True(t, doc.Settings.ShowLeaderboard)
	assert.Equal(t, game.RouteStrategyOrdered, doc.Structure.Routing)
	assert.Empty(t, doc.Structure.Children)
	assert.Nil(t, doc.Structure.ChildrenMin, "an unset band exports as omitted")
	assert.Nil(t, doc.Structure.ChildrenMax)
	assert.Empty(t, doc.Start)
	assert.Empty(t, doc.Finish)
}

func TestExportService_ExportInstance_WithObjectiveAndBlocks(t *testing.T) {
	svc, instanceRepo, settingsRepo, blockRepo, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Objective Export Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	root := &models.Objective{ID: gofakeit.UUID(), QuestID: inst.ID, Slug: "root", Title: "Root"}
	insertObjective(t, dbc, root)
	obj := &models.Objective{
		ID:         gofakeit.UUID(),
		QuestID:    inst.ID,
		ParentID:   root.ID,
		Slug:       "find-the-key",
		Title:      "Find the key",
		ProofSets:  game.SetsField{"door_unlocked"},
		RevealSets: game.SetsField{"story_seen"},
	}
	insertObjective(t, dbc, obj)

	transactor := db.NewTransactor(dbc)
	var err error
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	proofBlock, err := blockRepo.CreateTx(ctx, tx, newTextBlock(obj.ID), obj.ID, game.ContextObjectiveProof)
	require.NoError(t, err)
	revealBlock, err := blockRepo.CreateTx(ctx, tx, newTextBlock(obj.ID), obj.ID, game.ContextObjectiveReveal)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	doc, _, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	child := doc.Structure.Children[0]
	assert.Equal(t, obj.ID, child.ID)
	assert.Equal(t, "find-the-key", child.Slug)
	assert.Equal(t, "Find the key", child.Title)
	assert.Empty(t, child.Children, "an objective row exports as a leaf")

	require.Len(t, child.Proof.Blocks, 1)
	assert.Equal(t, proofBlock.GetID(), child.Proof.Blocks[0]["id"])
	assert.Equal(t, game.SetsField{"door_unlocked"}, child.Proof.Sets)

	require.Len(t, child.Reveal.Blocks, 1)
	assert.Equal(t, revealBlock.GetID(), child.Reveal.Blocks[0]["id"])
	assert.Equal(t, game.SetsField{"story_seen"}, child.Reveal.Sets)
}

func TestExportService_ExportInstance_StartFinishBlocks(t *testing.T) {
	svc, instanceRepo, settingsRepo, blockRepo, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Start Finish Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))
	insertObjective(t, dbc, &models.Objective{ID: gofakeit.UUID(), QuestID: inst.ID, Slug: "root", Title: "Root"})

	transactor := db.NewTransactor(dbc)
	tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	_, err = blockRepo.CreateTx(ctx, tx, newTextBlock(inst.ID), inst.ID, game.ContextStart)
	require.NoError(t, err)
	_, err = blockRepo.CreateTx(ctx, tx, newTextBlock(inst.ID), inst.ID, game.ContextFinish)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	doc, _, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Start, 1)
	assert.Equal(t, "text", doc.Start[0]["type"])
	require.Len(t, doc.Finish, 1)
	assert.Equal(t, "text", doc.Finish[0]["type"])
}

func TestExportService_ExportInstance_NestedSection(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Nested Game", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	root := &models.Objective{ID: gofakeit.UUID(), QuestID: inst.ID, Slug: "root", Title: "Root"}
	insertObjective(t, dbc, root)
	section := &models.Objective{
		ID:       gofakeit.UUID(),
		QuestID:  inst.ID,
		ParentID: root.ID,
		Slug:     "wave-1",
		Title:    "Wave 1",
		Color:    "primary",
		Routing:  game.RouteStrategyOrdered,
	}
	insertObjective(t, dbc, section)
	insertObjective(t, dbc, &models.Objective{
		ID: gofakeit.UUID(), QuestID: inst.ID, ParentID: section.ID, Slug: "spot", Title: "Spot",
	})

	doc, _, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	child := doc.Structure.Children[0]
	assert.Equal(t, section.ID, child.ID)
	assert.Equal(t, "wave-1", child.Slug)
	assert.Equal(t, "Wave 1", child.Title)
	assert.Equal(t, "primary", child.Color)
	require.Len(t, child.Children, 1)
	assert.Equal(t, "spot", child.Children[0].Slug)
}

func TestExportService_ObjectiveDependsRoundTrip(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Depends Test", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	root := &models.Objective{ID: gofakeit.UUID(), QuestID: inst.ID, Slug: "root", Title: "Root"}
	insertObjective(t, dbc, root)
	insertObjective(t, dbc, &models.Objective{
		ID:       gofakeit.UUID(),
		QuestID:  inst.ID,
		ParentID: root.ID,
		Slug:     "gated",
		Title:    "Gated Spot",
		Depends:  game.DependsField{"gate", "not decoy"},
	})

	doc, _, err := svc.ExportInstance(ctx, inst.ID)
	require.NoError(t, err)

	require.Len(t, doc.Structure.Children, 1)
	assert.Equal(t, game.DependsField{"gate", "not decoy"}, doc.Structure.Children[0].Depends)
}

// A quest with two parentless rows has no single tree. Picking one and dropping
// the rest exports a document missing whole subtrees, and re-importing it
// sweeps those objectives away as orphans, so the export is refused instead.
func TestExportService_ExportInstance_RefusesAmbiguousRoot(t *testing.T) {
	svc, instanceRepo, settingsRepo, _, dbc, cleanup := setupExportService(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)

	inst := &models.Quest{Name: "Two Roots", UserID: userID}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, settingsRepo.Create(ctx, &models.QuestSettings{QuestID: inst.ID}))

	insertObjective(t, dbc, &models.Objective{ID: gofakeit.UUID(), QuestID: inst.ID, Slug: "a", Title: "A"})
	insertObjective(t, dbc, &models.Objective{ID: gofakeit.UUID(), QuestID: inst.ID, Slug: "b", Title: "B"})

	_, _, err := svc.ExportInstance(ctx, inst.ID)
	require.ErrorIs(t, err, repositories.ErrAmbiguousRootObjective)
}
