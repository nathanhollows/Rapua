package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomTextRename_RenamesTypeAndListKey(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261010000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "random_clue", `{"clues":["First text","Second text"]}`)

	require.NoError(t, m20261010010000_up(context.Background(), dbc))

	blockType, data, found := blockRow(t, dbc, id)
	require.True(t, found, "the block keeps its row, so its place on the page is kept too")
	assert.Equal(t, "random_text", blockType)
	assert.JSONEq(t, `{"texts":["First text","Second text"]}`, data)
}

func TestRandomTextRename_LeavesClueBlocksAlone(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261010000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "clue", `{"clue":"Look near the fountain"}`)

	require.NoError(t, m20261010010000_up(context.Background(), dbc))

	blockType, data, found := blockRow(t, dbc, id)
	require.True(t, found)
	assert.Equal(t, "clue", blockType)
	assert.JSONEq(t, `{"clue":"Look near the fountain"}`, data)
}

func TestRandomTextRename_UnreadableDataStillGetsTheType(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261010000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "random_clue", `not json`)

	require.NoError(t, m20261010010000_up(context.Background(), dbc))

	blockType, data, found := blockRow(t, dbc, id)
	require.True(t, found)
	assert.Equal(t, "random_text", blockType)
	assert.Equal(t, "not json", data)
}

func TestRandomTextRename_DownRestoresTheOldNames(t *testing.T) {
	dbc := m20260827_setupDBThrough(t, "20261010000000")
	id := gofakeit.UUID()
	seedBlock(t, dbc, id, "random_clue", `{"clues":["One"]}`)

	require.NoError(t, m20261010010000_up(context.Background(), dbc))
	require.NoError(t, m20261010010000_down(context.Background(), dbc))

	blockType, data, found := blockRow(t, dbc, id)
	require.True(t, found)
	assert.Equal(t, "random_clue", blockType)
	assert.JSONEq(t, `{"clues":["One"]}`, data)
}
