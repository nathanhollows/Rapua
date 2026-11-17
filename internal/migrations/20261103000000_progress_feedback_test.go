package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressFeedbackMigration_AddsTheColumn(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261103000000")

	assert.True(t, columnExists(context.Background(), dbc, "quest_settings", "progress_feedback"))
}

func TestProgressFeedbackMigration_DownDropsIt(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261103000000")
	ctx := context.Background()

	require.NoError(t, m20261103000000_down(ctx, dbc))

	assert.False(t, columnExists(ctx, dbc, "quest_settings", "progress_feedback"))
}

// Both directions are guarded, so a retry after a partial failure finishes the
// job.
func TestProgressFeedbackMigration_IsRepeatable(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261103000000")
	ctx := context.Background()

	require.NoError(t, m20261103000000_up(ctx, dbc))
	require.NoError(t, m20261103000000_down(ctx, dbc))
	require.NoError(t, m20261103000000_down(ctx, dbc))
	require.NoError(t, m20261103000000_up(ctx, dbc))
	assert.True(t, columnExists(ctx, dbc, "quest_settings", "progress_feedback"))
}
