package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropShowTeamCount_ColumnGoneAfterFullMigration(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.False(t, columnExists(ctx, dbc, "quest_settings", "show_team_count"))
}

func TestDropShowTeamCount_UpIsIdempotent(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260905010000_up(ctx, dbc), "must be safe to run again once the column is already gone")
}

func TestDropShowTeamCount_DownRestoresColumn(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260905010000_down(ctx, dbc))
	require.True(t, columnExists(ctx, dbc, "quest_settings", "show_team_count"))
}
