package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// answeredSets reads what the stream says each answer set, in order.
func answeredSets(t *testing.T, f completingSubmission) [][]string {
	t.Helper()
	events, err := repositories.NewRunEventRepository(f.dbc).ListByRun(context.Background(), f.team.Code)
	require.NoError(t, err)
	var out [][]string
	for _, e := range events {
		if e.Type != models.EventBlockAnswered {
			continue
		}
		var payload models.BlockAnsweredData
		require.NoError(t, json.Unmarshal(e.Data, &payload))
		out = append(out, payload.Sets)
	}
	return out
}

func runVars(t *testing.T, f completingSubmission) []string {
	t.Helper()
	names, err := repositories.NewRunVariableRepository(f.dbc).FindNamesByRunCode(context.Background(), f.team.Code)
	require.NoError(t, err)
	return names
}

func giveSets(t *testing.T, f completingSubmission, block blocks.Block, name string) {
	t.Helper()
	block.SetVars(name, nil)
	_, err := f.blockService.UpdateBlock(context.Background(), block, map[string][]string{})
	require.NoError(t, err)
}

func TestCheckInService_CompletingABlockSetsItsVariable(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	giveSets(t, f, f.block, "met_guide")

	_, _, err := f.svc.ValidateAndUpdateBlockState(context.Background(), f.team, map[string][]string{
		"block": {f.block.GetID()}, "response": {"hello"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"met_guide"}, runVars(t, f))
	assert.Equal(t, [][]string{{"met_guide"}}, answeredSets(t, f))
}

// A block in an objective the run cannot reach still saves its state, so this
// is what keeps a player from opening a gate by posting to it directly.
func TestCheckInService_UnreachableBlockSetsNothing(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()
	giveSets(t, f, f.block, "spoke_to_guide")

	root := &models.Objective{ID: gofakeit.UUID(), QuestID: f.team.QuestID, Slug: "root", Title: "Root"}
	_, err := f.dbc.NewInsert().Model(root).Exec(ctx)
	require.NoError(t, err)
	_, err = f.dbc.NewUpdate().Model((*models.Objective)(nil)).
		Set("parent_id = ?", root.ID).
		Set("depends = ?", `["never_set"]`).
		Where("id = ?", f.objective.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "response": {"hello"},
	})
	require.NoError(t, err)

	assert.Empty(t, runVars(t, f))
	assert.Equal(t, [][]string{nil}, answeredSets(t, f))
}

func TestCheckInService_MultiSelectChoiceSetsEveryPick(t *testing.T) {
	f := setupCompletingSubmission(t, "choice", repositories.NewRunEventRepository)
	ctx := context.Background()
	_, err := f.blockService.UpdateBlock(ctx, f.block, map[string][]string{
		"prompt":       {"Where to?"},
		"multi_select": {"on"},
		"option_label": {"Forest", "River", "Hill"},
		"option_sets":  {"forest", "river", "hill"},
	})
	require.NoError(t, err)

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "choice": {"river", "forest"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"forest", "river"}, runVars(t, f))
}

// The stream adds up to the table: a variable a run already had is not
// announced again.
func TestCheckInService_SecondSetterAnnouncesNothing(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()
	giveSets(t, f, f.block, "met_guide")

	second, err := f.blockService.NewBlockWithOwnerAndContext(
		ctx,
		f.objective.ID,
		game.ContextObjectiveProof,
		"free_text",
	)
	require.NoError(t, err)
	giveSets(t, f, second, "met_guide")
	_, _, err = f.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, f.objective.ID, f.team.Code, f.team.QuestID, game.ContextObjectiveProof,
	)
	require.NoError(t, err)

	for _, id := range []string{f.block.GetID(), second.GetID()} {
		_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
			"block": {id}, "response": {"hello"},
		})
		require.NoError(t, err)
	}

	assert.Equal(t, []string{"met_guide"}, runVars(t, f))
	assert.Equal(t, [][]string{{"met_guide"}, nil}, answeredSets(t, f))
}
