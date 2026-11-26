package services_test

import (
	"context"
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

// scanQuest is a root holding a visible leaf and a secret, each proved by a
// scan worth ten points.
type scanQuest struct {
	svc     *services.CheckInService
	dbc     *bun.DB
	team    models.Run
	visible *models.Objective
	secret  *models.Objective
}

func setupScanQuest(t *testing.T) scanQuest {
	t.Helper()
	dbc, cleanup := setupDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := gofakeit.LetterN(6)
	insertTestTeam(t, dbc, runCode, parents.QuestID)

	insert := func(obj *models.Objective) *models.Objective {
		_, err := dbc.NewInsert().Model(obj).Exec(ctx)
		require.NoError(t, err)
		return obj
	}
	root := insert(&models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "root", Title: "Root",
		Routing: game.RouteStrategyFreeRoam,
	})
	visible := insert(&models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: root.ID, Position: 0,
		Slug: "library", Title: "Library", Routing: game.RouteStrategyOrdered,
	})
	secret := insert(&models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: root.ID, Position: 1,
		Slug: "rooftop", Title: "Rooftop", Routing: game.RouteStrategyOrdered, Secret: true,
	})

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	for obj, code := range map[*models.Objective]string{visible: "LIBRY", secret: "ROOFS"} {
		block, err := blockService.NewBlockWithOwnerAndContext(ctx, obj.ID, blocks.ContextObjectiveProof, "scan")
		require.NoError(t, err)
		_, err = dbc.NewUpdate().Model((*models.Block)(nil)).
			Set("data = ?", `{"prompt":"Scan","codes":[{"value":"`+code+`"}],"match":"contains","allow_typing":true}`).
			Set("points = 10").
			Where("id = ?", block.GetID()).Exec(ctx)
		require.NoError(t, err)
	}

	svc := services.NewCheckInService(
		db.NewTransactor(dbc),
		repositories.NewRunRepository(dbc),
		blockService,
		repositories.NewObjectiveRepository(dbc),
		repositories.NewObjectiveContextCompletionRepository(dbc),
		repositories.NewSectionFinishRepository(dbc),
		blockRepo,
		repositories.NewRunEventRepository(dbc),
		repositories.NewRunVariableRepository(dbc),
	)
	return scanQuest{
		svc: svc, dbc: dbc, team: models.Run{Code: runCode, QuestID: parents.QuestID},
		visible: visible, secret: secret,
	}
}

func (q scanQuest) points(t *testing.T) int {
	t.Helper()
	var run models.Run
	require.NoError(t, q.dbc.NewSelect().Model(&run).Where("code = ?", q.team.Code).Scan(context.Background()))
	return run.Points
}

func (q scanQuest) proofDone(t *testing.T, objectiveID string) bool {
	t.Helper()
	n, err := q.dbc.NewSelect().Model((*models.ObjectiveContextCompletion)(nil)).
		Where("run_code = ? AND objective_id = ? AND context = ?", q.team.Code, objectiveID, game.ContextObjectiveProof).
		Count(context.Background())
	require.NoError(t, err)
	return n > 0
}

func TestScanCode_FindsAVisibleObjective(t *testing.T) {
	q := setupScanQuest(t)

	found, err := q.svc.ScanCode(context.Background(), &q.team, "LIBRY", blocks.ModalityTyped)
	require.NoError(t, err)
	assert.Equal(t, q.visible.ID, found.ID)
	assert.True(t, q.proofDone(t, q.visible.ID))
	assert.Equal(t, 10, q.points(t))
}

func TestScanCode_DiscoversASecret(t *testing.T) {
	q := setupScanQuest(t)

	found, err := q.svc.ScanCode(context.Background(), &q.team, "https://rapua.nz/s/ROOFS", blocks.ModalityCamera)
	require.NoError(t, err)
	assert.Equal(t, q.secret.ID, found.ID)
	assert.True(t, q.proofDone(t, q.secret.ID), "the find is the proof")
	assert.Equal(t, 10, q.points(t), "and pays like any other")
}

func TestScanCode_NoMatch(t *testing.T) {
	q := setupScanQuest(t)

	_, err := q.svc.ScanCode(context.Background(), &q.team, "NOPE1", blocks.ModalityTyped)
	require.ErrorIs(t, err, services.ErrNoScanMatch)
	assert.Zero(t, q.points(t))
}

func TestScanCode_RescanReturnsTheObjectiveAndPaysNothing(t *testing.T) {
	q := setupScanQuest(t)
	ctx := context.Background()

	_, err := q.svc.ScanCode(ctx, &q.team, "ROOFS", blocks.ModalityTyped)
	require.NoError(t, err)
	again, err := q.svc.ScanCode(ctx, &q.team, "ROOFS", blocks.ModalityTyped)
	require.NoError(t, err)
	assert.Equal(t, q.secret.ID, again.ID, "an old poster still leads somewhere")
	assert.Equal(t, 10, q.points(t))
}

func TestScanCode_ClosedSecretIsNotFound(t *testing.T) {
	q := setupScanQuest(t)
	ctx := context.Background()

	// Finishing the visible leaf completes the root, which closes the secret.
	_, err := q.svc.ScanCode(ctx, &q.team, "LIBRY", blocks.ModalityTyped)
	require.NoError(t, err)

	_, err = q.svc.ScanCode(ctx, &q.team, "ROOFS", blocks.ModalityTyped)
	require.ErrorIs(t, err, services.ErrNoScanMatch)
	assert.Equal(t, 10, q.points(t))
}

// A completed discovery block is the find, so posting a closed secret's block
// directly must not save one.
func TestScanCode_ClosedSecretRefusesADirectSubmit(t *testing.T) {
	q := setupScanQuest(t)
	ctx := context.Background()

	_, err := q.svc.ScanCode(ctx, &q.team, "LIBRY", blocks.ModalityTyped)
	require.NoError(t, err)

	var blockID string
	require.NoError(t, q.dbc.NewSelect().Model((*models.Block)(nil)).Column("id").
		Where("owner_id = ?", q.secret.ID).Scan(ctx, &blockID))
	_, _, err = q.svc.ValidateAndUpdateBlockState(ctx, q.team, map[string][]string{
		"block": {blockID}, "code": {"ROOFS"},
	})
	require.ErrorIs(t, err, services.ErrSecretOutOfReach)

	complete, err := q.dbc.NewSelect().Table("run_block_states").
		Where("run_code = ? AND block_id = ? AND is_complete", q.team.Code, blockID).Exists(ctx)
	require.NoError(t, err)
	assert.False(t, complete)
}
