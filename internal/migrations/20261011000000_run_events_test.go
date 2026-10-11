package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// m20261011000000_indexes names the indexes the up creates, so both tests assert
// the same set.
func m20261011000000_indexes() []string {
	return []string{
		"idx_run_events_run_code_created_at",
		"idx_run_events_quest_id_created_at",
	}
}

func TestRunEventsMigration_CreatesTheStream(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261011000000")
	ctx := context.Background()

	assert.True(t, tableExists(ctx, dbc, "run_events"))
	for _, name := range m20261011000000_indexes() {
		assert.True(t, indexExists(ctx, dbc, name), "%s", name)
	}

	for _, column := range []string{
		"id", "run_code", "quest_id", "type", "objective_id", "block_id", "points", "created_at", "data",
	} {
		assert.True(t, columnExists(ctx, dbc, "run_events", column), "run_events.%s", column)
	}
}

func TestRunEventsMigration_DownDropsEverything(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261011000000")
	ctx := context.Background()

	require.NoError(t, m20261011000000_down(ctx, dbc))

	assert.False(t, tableExists(ctx, dbc, "run_events"))
	for _, name := range m20261011000000_indexes() {
		assert.False(t, indexExists(ctx, dbc, name), "%s", name)
	}
}

// Both directions are guarded on what already exists, so a retry after a partial
// failure finishes the job instead of erroring on the half already done.
func TestRunEventsMigration_IsRepeatable(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261011000000")
	ctx := context.Background()

	require.NoError(t, m20261011000000_up(ctx, dbc), "up over an already-migrated schema")
	require.NoError(t, m20261011000000_down(ctx, dbc))
	require.NoError(t, m20261011000000_down(ctx, dbc), "down over an already-reverted schema")
	require.NoError(t, m20261011000000_up(ctx, dbc))

	assert.True(t, tableExists(ctx, dbc, "run_events"))
	for _, name := range m20261011000000_indexes() {
		assert.True(t, indexExists(ctx, dbc, name), "%s", name)
	}
}
