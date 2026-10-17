package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectivePassThroughMigration_AddsTheColumn(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261017000000")

	assert.True(t, columnExists(context.Background(), dbc, "objectives", "pass_through"))
}

func TestObjectivePassThroughMigration_DownDropsIt(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261017000000")
	ctx := context.Background()

	require.NoError(t, m20261017000000_down(ctx, dbc))

	assert.False(t, columnExists(ctx, dbc, "objectives", "pass_through"))
}

// Both directions are guarded on what already exists, so a retry after a partial
// failure finishes the job instead of erroring on the half already done.
func TestObjectivePassThroughMigration_IsRepeatable(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261017000000")
	ctx := context.Background()

	require.NoError(t, m20261017000000_up(ctx, dbc))
	require.NoError(t, m20261017000000_down(ctx, dbc))
	require.NoError(t, m20261017000000_down(ctx, dbc))
	require.NoError(t, m20261017000000_up(ctx, dbc))
	assert.True(t, columnExists(ctx, dbc, "objectives", "pass_through"))
}
