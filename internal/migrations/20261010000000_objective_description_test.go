package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectiveDescription_AddsTheColumn(t *testing.T) {
	dbc := m20260827_setupDB(t)

	assert.True(t, columnExists(context.Background(), dbc, "objectives", "description"))
}

// Existing rows get an empty description rather than a null one, so every
// reader can treat it as a string and none has to decide what a missing
// description means.
func TestObjectiveDescription_IsEmptyNotNull(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	_, err := dbc.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ('u','a@b.c')`)
	require.NoError(t, err)
	_, err = dbc.ExecContext(ctx,
		`INSERT INTO quests (id, user_id, name) VALUES ('q','u','Q')`)
	require.NoError(t, err)
	_, err = dbc.ExecContext(ctx,
		`INSERT INTO objectives (id, quest_id, slug, title, routing) VALUES ('o','q','s','T','ordered')`)
	require.NoError(t, err)

	var description string
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT description FROM objectives WHERE id = 'o'`).Scan(&description))
	assert.Empty(t, description)
}

func TestObjectiveDescription_DownRemovesIt(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261010000000")
	ctx := context.Background()

	require.NoError(t, m20261010000000_down(ctx, dbc))
	assert.False(t, columnExists(ctx, dbc, "objectives", "description"))
}
