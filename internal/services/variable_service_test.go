package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockService_UpdateBlockVars(t *testing.T) {
	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()
	owner := gofakeit.UUID()

	newBlock := func(blockType string) blocks.Block {
		b, err := svc.NewBlockWithOwnerAndContext(ctx, owner, blocks.ContextObjectiveProof, blockType)
		require.NoError(t, err)
		return b
	}

	t.Run("an interactive block sets one", func(t *testing.T) {
		b, err := svc.UpdateBlockVars(ctx, newBlock("free_text"), strPtr("met_guide"), nil)
		require.NoError(t, err)
		reloaded, err := svc.GetByBlockID(ctx, b.GetID())
		require.NoError(t, err)
		assert.Equal(t, "met_guide", reloaded.GetSets())
	})

	t.Run("a content block waits on some, tidied", func(t *testing.T) {
		deps := []string{"", "key", "lamp", "key"}
		b, err := svc.UpdateBlockVars(ctx, newBlock("text"), nil, &deps)
		require.NoError(t, err)
		assert.Equal(t, game.DependsField{"key", "lamp"}, b.GetDepends())

		cleared := []string{""}
		b, err = svc.UpdateBlockVars(ctx, b, nil, &cleared)
		require.NoError(t, err)
		assert.Empty(t, b.GetDepends(), "the blank the form always sends clears the gate")
	})

	refusals := []struct {
		name      string
		blockType string
		sets      *string
		depends   *[]string
		err       error
	}{
		{"content cannot set", "text", strPtr("a"), nil, services.ErrVarsNotAllowed},
		{"a choice sets per option", "choice", strPtr("a"), nil, services.ErrVarsNotAllowed},
		{"an answer cannot wait", "free_text", nil, &[]string{"a"}, services.ErrVarsNotAllowed},
		{"a bad name", "free_text", strPtr("Met-Guide"), nil, services.ErrInvalidVarName},
	}
	for _, c := range refusals {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.UpdateBlockVars(ctx, newBlock(c.blockType), c.sets, c.depends)
			require.ErrorIs(t, err, c.err)
		})
	}
}

func TestVariableService_ListsAndSummarisesTheQuest(t *testing.T) {
	blockSvc, dbc, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "gate", Title: "Gate"}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	answer, err := blockSvc.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)
	_, err = blockSvc.UpdateBlockVars(ctx, answer, strPtr("met_guide"), nil)
	require.NoError(t, err)

	choice, err := blockSvc.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveProof, "choice")
	require.NoError(t, err)
	_, err = blockSvc.UpdateBlock(ctx, choice, map[string][]string{
		"prompt": {"Which way?"}, "option_label": {"Left", "Right"}, "option_sets": {"left", "right"},
	})
	require.NoError(t, err)

	secret, err := blockSvc.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveReveal, "text")
	require.NoError(t, err)
	_, err = blockSvc.UpdateBlockVars(ctx, secret, nil, &[]string{"left"})
	require.NoError(t, err)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	svc := services.NewVariableService(
		repositories.NewObjectiveRepository(dbc),
		repositories.NewBlockRepository(dbc, blockStateRepo),
	)

	names, err := svc.QuestVariables(ctx, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, []string{"left", "met_guide", "right"}, names)

	usage, err := svc.UsageByOwner(ctx, parents.QuestID)
	require.NoError(t, err)
	u := usage[objective.ID]
	require.Len(t, u.Sets, 2)
	require.Len(t, u.Gated, 1)
	assert.Equal(t, secret.GetID(), u.Gated[0].BlockID)
	assert.Equal(t, []string{"left"}, u.Gated[0].Vars)
}
