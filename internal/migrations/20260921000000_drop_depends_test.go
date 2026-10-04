package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The columns go rather than being left unmapped: a column nothing reads
// invites a later reader to wonder whether the feature is still there.
func TestDropDepends_RemovesTheConditionalSystem(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260907000000")
	ctx := context.Background()

	for _, column := range []string{"depends", "proof_sets", "reveal_sets"} {
		require.True(t, columnExists(ctx, dbc, "objectives", column),
			"%s is there before the migration", column)
	}

	require.NoError(t, m20260921000000_up(ctx, dbc))

	for _, column := range []string{"depends", "proof_sets", "reveal_sets"} {
		assert.False(t, columnExists(ctx, dbc, "objectives", column), "%s is gone", column)
	}
	assert.False(t, tableExists(ctx, dbc, "run_var_states"),
		"the variables had no reader left once depends went")
}

// Running it twice must not fail: a migration that cannot be re-run is one
// nobody can recover from halfway.
func TestDropDepends_IsIdempotent(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260907000000")
	ctx := context.Background()

	require.NoError(t, m20260921000000_up(ctx, dbc))
	assert.NoError(t, m20260921000000_up(ctx, dbc))
}

// Down restores the shape so a rollback leaves a schema the older code can
// open, even though the gates themselves are not coming back.
func TestDropDepends_DownRestoresTheColumns(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260907000000")
	ctx := context.Background()

	require.NoError(t, m20260921000000_up(ctx, dbc))
	require.NoError(t, m20260921000000_down(ctx, dbc))

	for _, column := range []string{"depends", "proof_sets", "reveal_sets"} {
		assert.True(t, columnExists(ctx, dbc, "objectives", column), "%s is back", column)
	}
}
