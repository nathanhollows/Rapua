package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// seedRoutingQuest writes a quest with the given window and one objective
// carrying no routing, which is the state the backfill left behind.
func seedRoutingQuest(t *testing.T, dbc *bun.DB, objectiveID string, start, end time.Time) {
	t.Helper()
	ctx := context.Background()
	questID := gofakeit.UUID()

	_, err := dbc.ExecContext(ctx,
		`INSERT INTO "quests" ("id", "name", "start_time", "end_time") VALUES (?, ?, ?, ?)`,
		questID, "Q", start, end)
	require.NoError(t, err)
	_, err = dbc.ExecContext(ctx,
		`INSERT INTO "objectives" ("id", "quest_id", "position", "slug", "title", "routing")`+
			` VALUES (?, ?, 0, ?, 'Root', '')`, objectiveID, questID, gofakeit.UUID())
	require.NoError(t, err)
}

func routingOf(t *testing.T, dbc *bun.DB, objectiveID string) string {
	t.Helper()
	var routing string
	require.NoError(t, dbc.QueryRowContext(context.Background(),
		`SELECT "routing" FROM "objectives" WHERE "id" = ?`, objectiveID).Scan(&routing))
	return routing
}

// Every objective ends up with a strategy, including one on a quest that is
// still open: a game left running with no end time would otherwise keep its
// empty routing forever, which is the state this exists to clear.
func TestRequireRouting_BackfillsEveryObjective(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260905010000")

	closed, running := gofakeit.UUID(), gofakeit.UUID()
	seedRoutingQuest(t, dbc, closed, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	seedRoutingQuest(t, dbc, running, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	require.NoError(t, m20260906000000_up(context.Background(), dbc))
	assert.Equal(t, "ordered", routingOf(t, dbc, closed))
	assert.Equal(t, "ordered", routingOf(t, dbc, running), "a running quest is converted too")
}

// The retired "secret" strategy is converted like anything else invalid: the
// engine was already playing those rows as free roam while the linter called
// them an error, so leaving them would be a permanent red mark nobody could
// clear from the editor.
func TestRequireRouting_ConvertsRetiredAndUnknownValues(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260905010000")
	ctx := context.Background()

	for _, stored := range []string{"secret", "nonsense", ""} {
		objectiveID := gofakeit.UUID()
		seedRoutingQuest(t, dbc, objectiveID, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
		_, err := dbc.ExecContext(ctx,
			`UPDATE "objectives" SET "routing" = ? WHERE "id" = ?`, stored, objectiveID)
		require.NoError(t, err)

		require.NoError(t, m20260906000000_up(ctx, dbc))
		assert.Equal(t, "ordered", routingOf(t, dbc, objectiveID), "was %q", stored)
	}
}

// A strategy an author actually chose is left alone.
func TestRequireRouting_LeavesValidStrategiesAlone(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260905010000")
	ctx := context.Background()

	objectiveID := gofakeit.UUID()
	seedRoutingQuest(t, dbc, objectiveID, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	_, err := dbc.ExecContext(ctx,
		`UPDATE "objectives" SET "routing" = 'free_roam' WHERE "id" = ?`, objectiveID)
	require.NoError(t, err)

	require.NoError(t, m20260906000000_up(ctx, dbc))
	assert.Equal(t, "free_roam", routingOf(t, dbc, objectiveID))
}
