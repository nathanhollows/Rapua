package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Roots written by the backfill hold a literal empty string, which is a real
// value to the unique index, so two of them at position 0 collide where two
// NULLs would not.
func TestNormaliseRootParent(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260904000000")
	ctx := context.Background()

	questA, questB := gofakeit.UUID(), gofakeit.UUID()
	for i, questID := range []string{questA, questB} {
		_, err := dbc.ExecContext(ctx, `INSERT INTO "quests" ("id", "name") VALUES (?, ?)`, questID, "Q")
		require.NoError(t, err)
		// Positions differ because the collision is exactly what forced them
		// apart in the first place.
		_, err = dbc.ExecContext(ctx,
			`INSERT INTO "objectives" ("id", "quest_id", "parent_id", "position", "slug", "title")`+
				` VALUES (?, ?, '', ?, 'root', 'Root')`, gofakeit.UUID(), questID, i)
		require.NoError(t, err)
	}

	require.NoError(t, m20260905000000_up(ctx, dbc))

	var empties, positions int
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM "objectives" WHERE "parent_id" = ''`).Scan(&empties))
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM "objectives" WHERE "parent_id" IS NULL AND "position" = 0`).Scan(&positions))

	assert.Equal(t, 0, empties, "no parent has one spelling")
	assert.Equal(t, 2, positions, "and both roots sit at 0, which NULLs allow")
}
