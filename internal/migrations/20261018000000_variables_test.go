package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesMigration_AddsColumnAndTable(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261018000000")
	ctx := context.Background()

	assert.True(t, columnExists(ctx, dbc, "objectives", "depends"))
	assert.True(t, tableExists(ctx, dbc, "run_variables"))
}

func TestVariablesMigration_DownDropsBoth(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261018000000")
	ctx := context.Background()

	require.NoError(t, m20261018000000_down(ctx, dbc))

	assert.False(t, columnExists(ctx, dbc, "objectives", "depends"))
	assert.False(t, tableExists(ctx, dbc, "run_variables"))
}

// Both directions are guarded on what already exists, so a retry after a partial
// failure finishes the job instead of erroring on the half already done.
func TestVariablesMigration_IsRepeatable(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261018000000")
	ctx := context.Background()

	require.NoError(t, m20261018000000_up(ctx, dbc))
	require.NoError(t, m20261018000000_down(ctx, dbc))
	require.NoError(t, m20261018000000_down(ctx, dbc))
	require.NoError(t, m20261018000000_up(ctx, dbc))
	assert.True(t, columnExists(ctx, dbc, "objectives", "depends"))
	assert.True(t, tableExists(ctx, dbc, "run_variables"))
}
