package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

// A clue is paid for: its points are stored negative, revealing it takes them
// off the run's score, and the event stream records the same deduction so the
// stream still sums to the score. Revealing it again must not charge twice.
func TestCheckInService_ClueRevealDeductsItsCost(t *testing.T) {
	f := setupCompletingSubmission(t, "clue", repositories.NewRunEventRepository)
	ctx := context.Background()

	const cost = -15
	_, err := f.dbc.NewUpdate().
		Model((*models.Block)(nil)).
		Set("points = ?", cost).
		Where("id = ?", f.block.GetID()).
		Exec(ctx)
	require.NoError(t, err)

	// The run starts with points to spend, so a deduction is visible as a drop
	// and not as a figure that happens to be negative.
	const before = 20
	_, err = f.dbc.NewUpdate().
		Model((*models.Run)(nil)).
		Set("points = ?", before).
		Where("code = ?", f.team.Code).
		Exec(ctx)
	require.NoError(t, err)

	reveal := map[string][]string{"block": {f.block.GetID()}, "reveal_clue": {"true"}}
	score := func() int {
		var run models.Run
		require.NoError(t, f.dbc.NewSelect().Model(&run).Where("code = ?", f.team.Code).Scan(ctx))
		return run.Points
	}

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, reveal)
	require.NoError(t, err)
	assert.Equal(t, before+cost, score(), "the clue's cost came off the score")

	stored, err := repositories.NewRunEventRepository(f.dbc).ListByRun(ctx, f.team.Code)
	require.NoError(t, err)
	streamed := 0
	for _, e := range stored {
		streamed += e.Points
	}
	assert.Equal(t, cost, streamed, "the stream records the deduction, not a credit")

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, reveal)
	require.NoError(t, err)
	assert.Equal(t, before+cost, score(), "revealing it again does not charge twice")
}
