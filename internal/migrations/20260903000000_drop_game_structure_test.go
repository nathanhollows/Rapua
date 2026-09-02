package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDropGameStructure_DropsBothColumns(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260902000000")
	ctx := context.Background()

	require.True(t, columnExists(ctx, dbc, "quests", "game_structure"))
	require.True(t, columnExists(ctx, dbc, "runs", "skipped_group_ids"))

	require.NoError(t, m20260903000000_up(ctx, dbc))

	assert.False(t, columnExists(ctx, dbc, "quests", "game_structure"))
	assert.False(t, columnExists(ctx, dbc, "runs", "skipped_group_ids"))
}

// The migration ships after the columns are already gone from a fresh schema,
// so it has to survive finding nothing to drop.
func TestDropGameStructure_RerunIsANoOp(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260902000000")
	ctx := context.Background()

	require.NoError(t, m20260903000000_up(ctx, dbc))
	require.NoError(t, m20260903000000_up(ctx, dbc))
}

func TestDropGameStructure_DownRestoresThemEmpty(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260902000000")
	ctx := context.Background()

	require.NoError(t, m20260903000000_up(ctx, dbc))
	require.NoError(t, m20260903000000_down(ctx, dbc))

	assert.True(t, columnExists(ctx, dbc, "quests", "game_structure"))
	assert.True(t, columnExists(ctx, dbc, "runs", "skipped_group_ids"))

	// Restoring twice must not fail on the column already being back.
	require.NoError(t, m20260903000000_down(ctx, dbc))
}
