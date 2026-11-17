package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func seedBlock(t *testing.T, dbc *bun.DB, id, blockType, data string) {
	t.Helper()
	_, err := dbc.ExecContext(context.Background(),
		`INSERT INTO "blocks" ("id", "owner_id", "type", "context", "data", "ordering", "points")`+
			` VALUES (?, ?, ?, 'objective_proof', ?, 0, 0)`,
		id, gofakeit.UUID(), blockType, data)
	require.NoError(t, err)
}

func blockRow(t *testing.T, dbc *bun.DB, id string) (string, string, bool) {
	t.Helper()
	var blockType, data string
	err := dbc.QueryRowContext(context.Background(),
		`SELECT "type", "data" FROM "blocks" WHERE "id" = ?`, id).Scan(&blockType, &data)
	if err != nil {
		return "", "", false
	}
	return blockType, data, true
}

func TestConvertRetiredBlocks_TaskKeepsItsWordsAsAChecklist(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260906000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "task",
		`{"task":"Photograph your door clear tag","icon":"camera","link_through":true}`)

	require.NoError(t, m20260907000000_up(context.Background(), dbc))

	blockType, data, found := blockRow(t, dbc, id)
	require.True(t, found, "the block keeps its row, so its place on the page is kept too")
	assert.Equal(t, "checklist", blockType)

	var checklist struct {
		Items []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Checked     bool   `json:"checked"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(data), &checklist))
	require.Len(t, checklist.Items, 1, "a task is one thing to do")
	assert.Equal(t, "Photograph your door clear tag", checklist.Items[0].Description)
	assert.NotEmpty(t, checklist.Items[0].ID, "the item needs an id to be ticked")
	assert.False(t, checklist.Items[0].Checked)
}

func TestConvertRetiredBlocks_TaskGatesAndKeepsItsTicks(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260906000000")
	ctx := context.Background()

	user := gofakeit.UUID()
	quest := gofakeit.UUID()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO "users" ("id", "name", "email") VALUES (?, ?, ?)`, []any{user, gofakeit.Name(), gofakeit.Email()}},
		{`INSERT INTO "quests" ("id", "name", "user_id") VALUES (?, 'Tasks', ?)`, []any{quest, user}},
	} {
		_, err := dbc.ExecContext(ctx, stmt.sql, stmt.args...)
		require.NoError(t, err)
	}
	done := m20260827_seedRun(t, dbc, quest)
	pending := m20260827_seedRun(t, dbc, quest)

	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "task", `{"task":"Find the bell"}`)
	for _, run := range []struct {
		code     string
		complete bool
	}{{done, true}, {pending, false}} {
		_, err := dbc.ExecContext(ctx,
			`INSERT INTO "run_block_states" ("run_code", "block_id", "quest_id", "is_complete", "points_awarded")`+
				` VALUES (?, ?, ?, ?, 0)`, run.code, id, quest, run.complete)
		require.NoError(t, err)
	}

	require.NoError(t, m20260907000000_up(ctx, dbc))

	var validation bool
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT "validation_required" FROM "blocks" WHERE "id" = ?`, id).Scan(&validation))
	assert.True(t, validation, "a checklist requires validation, and duplication copies the stored flag")

	_, data, _ := blockRow(t, dbc, id)
	var checklist struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(data), &checklist))
	require.Len(t, checklist.Items, 1)

	stateOf := func(runCode string) string {
		var playerData *string
		require.NoError(t, dbc.QueryRowContext(ctx,
			`SELECT "player_data" FROM "run_block_states" WHERE "run_code" = ? AND "block_id" = ?`,
			runCode, id).Scan(&playerData))
		if playerData == nil {
			return ""
		}
		return *playerData
	}
	assert.JSONEq(t, `{"checked_items":["`+checklist.Items[0].ID+`"]}`, stateOf(done))
	assert.Empty(t, stateOf(pending), "an unfinished task has nothing to tick")
}

func TestConvertRetiredBlocks_StructuralResidueIsDeleted(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260906000000")
	listID, slotID := gofakeit.UUID(), gofakeit.UUID()
	seedBlock(t, dbc, listID, "location_list", `{}`)
	seedBlock(t, dbc, slotID, "location_slot", `{}`)

	require.NoError(t, m20260907000000_up(context.Background(), dbc))

	for _, id := range []string{listID, slotID} {
		_, _, found := blockRow(t, dbc, id)
		assert.False(t, found, "nothing rendered it and nothing wrote it")
	}
}

func TestConvertRetiredBlocks_LeavesLiveBlocksAlone(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260906000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "text", `{"content":"Read this"}`)

	require.NoError(t, m20260907000000_up(context.Background(), dbc))

	blockType, data, found := blockRow(t, dbc, id)
	require.True(t, found)
	assert.Equal(t, "text", blockType)
	assert.JSONEq(t, `{"content":"Read this"}`, data)
}

func TestConvertRetiredBlocks_EmptyTaskStillConverts(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20260906000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "task", `{}`)

	require.NoError(t, m20260907000000_up(context.Background(), dbc))

	blockType, _, found := blockRow(t, dbc, id)
	require.True(t, found)
	assert.Equal(t, "checklist", blockType)
}
