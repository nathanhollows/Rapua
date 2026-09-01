package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// m20260901010000_seedQuest inserts a bare quest and run. The sibling
// migration's fixture builds a whole location tree and needs tables a later
// migration drops; these tests only need something to hang blocks off.
func m20260901010000_seedQuest(t *testing.T, dbc *bun.DB) (string, string) {
	t.Helper()
	ctx := context.Background()
	questID, runCode := gofakeit.UUID(), gofakeit.LetterN(6)

	_, err := dbc.ExecContext(ctx, `INSERT INTO "quests" ("id", "name") VALUES (?, 'Broker Test')`, questID)
	require.NoError(t, err)
	_, err = dbc.ExecContext(ctx, `INSERT INTO "runs" ("code", "quest_id") VALUES (?, ?)`, runCode, questID)
	require.NoError(t, err)

	return questID, runCode
}

// m20260901010000_seedBlock inserts one block and a run state against it,
// bypassing the models so the removed broker type can still be written.
func m20260901010000_seedBlock(t *testing.T, dbc *bun.DB, questID, runCode, blockType string) string {
	t.Helper()
	ctx := context.Background()
	blockID := gofakeit.UUID()

	_, err := dbc.ExecContext(ctx,
		`INSERT INTO "blocks" ("id", "owner_id", "type", "context", "data", "ordering", "points")`+
			` VALUES (?, ?, ?, 'objective_proof', '{}', 0, 0)`,
		blockID, questID, blockType)
	require.NoError(t, err)

	_, err = dbc.ExecContext(ctx,
		`INSERT INTO "run_block_states" ("run_code", "block_id", "quest_id", "is_complete", "points_awarded")`+
			` VALUES (?, ?, ?, 0, 0)`,
		runCode, blockID, questID)
	require.NoError(t, err)

	return blockID
}

func m20260901010000_countRows(t *testing.T, dbc *bun.DB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, dbc.NewRaw(query, args...).Scan(context.Background(), &count))
	return count
}

// A surviving broker row cannot be loaded into a block at all now that the type
// has left the registry, so the objective holding it would fail to render.
func TestDropBrokerBlocks_RemovesBrokersAndTheirState(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	questID, runCode := m20260901010000_seedQuest(t, dbc)

	brokerID := m20260901010000_seedBlock(t, dbc, questID, runCode, "broker")
	textID := m20260901010000_seedBlock(t, dbc, questID, runCode, "text")

	require.NoError(t, m20260901010000_up(ctx, dbc))

	assert.Zero(t,
		m20260901010000_countRows(t, dbc, `SELECT COUNT(*) FROM "blocks" WHERE "id" = ?`, brokerID),
		"the broker block is gone")
	assert.Zero(t,
		m20260901010000_countRows(t, dbc, `SELECT COUNT(*) FROM "run_block_states" WHERE "block_id" = ?`, brokerID),
		"its player state goes with it")

	assert.Equal(t, 1,
		m20260901010000_countRows(t, dbc, `SELECT COUNT(*) FROM "blocks" WHERE "id" = ?`, textID),
		"every other block is untouched")
	assert.Equal(t, 1,
		m20260901010000_countRows(t, dbc, `SELECT COUNT(*) FROM "run_block_states" WHERE "block_id" = ?`, textID),
		"and so is their state")
}

// An upload is a player's own file. It outlives the block it was made against
// rather than being deleted with it, so it only loses the reference.
func TestDropBrokerBlocks_DetachesUploadsRatherThanDeletingThem(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	questID, runCode := m20260901010000_seedQuest(t, dbc)

	brokerID := m20260901010000_seedBlock(t, dbc, questID, runCode, "broker")
	uploadID := gofakeit.UUID()
	_, err := dbc.ExecContext(ctx,
		`INSERT INTO "uploads" ("id", "quest_id", "block_id", "original_url", "storage")`+
			` VALUES (?, ?, ?, 'https://example.test/photo.jpg', 'local')`,
		uploadID, questID, brokerID)
	require.NoError(t, err)

	require.NoError(t, m20260901010000_up(ctx, dbc))

	assert.Equal(t, 1,
		m20260901010000_countRows(t, dbc, `SELECT COUNT(*) FROM "uploads" WHERE "id" = ?`, uploadID),
		"the file survives its block")
	assert.Zero(t,
		m20260901010000_countRows(t, dbc,
			`SELECT COUNT(*) FROM "uploads" WHERE "id" = ? AND "block_id" IS NOT NULL`, uploadID),
		"but no longer points at it")
}

// The overwhelmingly common case is a database that never held one, and a
// rerun after a partial failure must not care either.
func TestDropBrokerBlocks_NoBrokersIsANoOp(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260901010000_up(ctx, dbc))
	require.NoError(t, m20260901010000_up(ctx, dbc))
	require.NoError(t, m20260901010000_down(ctx, dbc))
}
