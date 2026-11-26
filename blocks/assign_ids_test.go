package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parsed[T blocks.Block](t *testing.T, block T, data string) T {
	t.Helper()
	switch b := any(block).(type) {
	case *blocks.SortingBlock:
		b.Data = json.RawMessage(data)
	case *blocks.ChecklistBlock:
		b.Data = json.RawMessage(data)
	case *blocks.QuizBlock:
		b.Data = json.RawMessage(data)
	}
	require.NoError(t, block.ParseData())
	return block
}

func TestSortingBlock_AssignMissingIDs(t *testing.T) {
	b := parsed(
		t,
		&blocks.SortingBlock{},
		`{"items":[{"description":"One","position":1},{"id":"keep","description":"Two","position":2},{"description":"Three","position":3}]}`,
	)

	require.NoError(t, b.AssignMissingIDs())

	ids := map[string]bool{}
	for _, item := range b.Items {
		assert.NotEmpty(t, item.ID)
		ids[item.ID] = true
	}
	assert.Len(t, ids, 3, "items that shared an empty id are told apart")
	assert.Equal(t, "keep", b.Items[1].ID, "an id the document gave is kept")
}

func TestChecklistBlock_AssignMissingIDs(t *testing.T) {
	b := parsed(t, &blocks.ChecklistBlock{},
		`{"items":[{"description":"A"},{"description":"B"}]}`)

	require.NoError(t, b.AssignMissingIDs())

	assert.NotEmpty(t, b.List[0].ID)
	assert.NotEqual(t, b.List[0].ID, b.List[1].ID)
}

func TestQuizBlock_AssignMissingIDs(t *testing.T) {
	b := parsed(t, &blocks.QuizBlock{},
		`{"question":"Q","options":[{"text":"A","correct":true},{"text":"B","correct":false}]}`)

	require.NoError(t, b.AssignMissingIDs())

	assert.Equal(t, "option_0", b.Options[0].ID, "named by place, as the editor does")
	assert.Equal(t, "option_1", b.Options[1].ID)
}
