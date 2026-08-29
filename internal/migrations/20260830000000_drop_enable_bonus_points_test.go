package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropEnableBonusPoints_ColumnGoneAfterFullMigration(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.False(t, columnExists(ctx, dbc, "quest_settings", "enable_bonus_points"))
}

func TestDropEnableBonusPoints_UpIsIdempotent(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260830000000_up(ctx, dbc), "must be safe to run again once the column is already gone")
}

func TestDropEnableBonusPoints_DownRestoresColumn(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260830000000_down(ctx, dbc))
	require.True(t, columnExists(ctx, dbc, "quest_settings", "enable_bonus_points"))

	// Idempotent the other direction too.
	require.NoError(t, m20260830000000_down(ctx, dbc))
}
