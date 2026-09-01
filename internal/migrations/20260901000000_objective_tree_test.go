package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// m20260901000000_addedColumns names what the up adds, so the assertions below
// cannot drift from the migration's own list.
func m20260901000000_addedColumns() []string {
	names := make([]string, 0, len(m20260901000000_objectiveColumns))
	for _, col := range m20260901000000_objectiveColumns {
		names = append(names, col.name)
	}
	return names
}

// The full auto-migrate in setupDB has already run this, so the up path is
// asserted against its real result rather than a hand-built schema.
func TestObjectiveTreeMigration_AddsTheTreeColumns(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	for _, name := range m20260901000000_addedColumns() {
		assert.True(t, columnExists(ctx, dbc, "objectives", name), "objectives.%s", name)
	}
	assert.False(t, columnExists(ctx, dbc, "objectives", "order"), "position replaces order")
	assert.True(t, tableExists(ctx, dbc, "section_finishes"))
}

func TestObjectiveTreeMigration_DownRestoresTheOldShape(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260901000000_down(ctx, dbc))

	for _, name := range m20260901000000_addedColumns() {
		assert.False(t, columnExists(ctx, dbc, "objectives", name), "objectives.%s", name)
	}
	assert.True(t, columnExists(ctx, dbc, "objectives", "order"), "order comes back")
	assert.False(t, tableExists(ctx, dbc, "section_finishes"))
}

// Both directions are guarded on what already exists, so a retry after a
// partial failure finishes the job instead of erroring on the half already done.
func TestObjectiveTreeMigration_IsRepeatable(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260901000000_up(ctx, dbc), "up over an already-migrated schema")
	require.NoError(t, m20260901000000_down(ctx, dbc))
	require.NoError(t, m20260901000000_down(ctx, dbc), "down over an already-reverted schema")
	require.NoError(t, m20260901000000_up(ctx, dbc))

	for _, name := range m20260901000000_addedColumns() {
		assert.True(t, columnExists(ctx, dbc, "objectives", name), "objectives.%s", name)
	}
	assert.True(t, tableExists(ctx, dbc, "section_finishes"))
}
