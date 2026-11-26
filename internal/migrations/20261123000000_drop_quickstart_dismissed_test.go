package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDropQuickstartDismissed_DropsTheColumn(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261123000000")

	assert.False(t, columnExists(context.Background(), dbc, "quests", "is_quick_start_dismissed"))
}

func TestDropQuickstartDismissed_DownRestoresIt(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261123000000")
	ctx := context.Background()

	require.NoError(t, m20261123000000_down(ctx, dbc))

	assert.True(t, columnExists(ctx, dbc, "quests", "is_quick_start_dismissed"))
}

func TestDropQuickstartDismissed_IsRepeatable(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261123000000")
	ctx := context.Background()

	require.NoError(t, m20261123000000_up(ctx, dbc))
	require.NoError(t, m20261123000000_down(ctx, dbc))
	require.NoError(t, m20261123000000_down(ctx, dbc))
	require.NoError(t, m20261123000000_up(ctx, dbc))
	assert.False(t, columnExists(ctx, dbc, "quests", "is_quick_start_dismissed"))
}
