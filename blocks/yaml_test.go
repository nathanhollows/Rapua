package blocks_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- FromYAML ---

func TestFromYAML_UnknownType(t *testing.T) {
	_, err := blocks.FromYAML("nonexistent_block", map[string]any{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, blocks.ErrBlockTypeNotFound))
}

func TestFromYAML_SimpleBlock_PassesThroughFields(t *testing.T) {
	fields := map[string]any{
		"content": "Hello world",
		"style":   "info",
	}
	data, err := blocks.FromYAML("alert", fields)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	assert.Equal(t, "Hello world", result["content"])
	assert.Equal(t, "info", result["style"])
}

// --- quizFromYAML ---

func TestFromYAML_Quiz_InjectsIDsAndOrder(t *testing.T) {
	fields := map[string]any{
		"question": "What is 2+2?",
		"options": []any{
			map[string]any{"text": "3", "correct": false},
			map[string]any{"text": "4", "correct": true},
		},
	}

	data, err := blocks.FromYAML("quiz", fields)
	require.NoError(t, err)

	var block blocks.QuizBlock
	block.BaseBlock.Data = data
	require.NoError(t, block.ParseData())

	require.Len(t, block.Options, 2)
	assert.Equal(t, "What is 2+2?", block.Question)

	assert.NotEmpty(t, block.Options[0].ID, "option 0 should have a generated ID")
	assert.NotEmpty(t, block.Options[1].ID, "option 1 should have a generated ID")
	assert.NotEqual(t, block.Options[0].ID, block.Options[1].ID, "IDs must be unique")

	assert.Equal(t, 0, block.Options[0].Order)
	assert.Equal(t, 1, block.Options[1].Order)

	assert.Equal(t, "3", block.Options[0].Text)
	assert.False(t, block.Options[0].IsCorrect)
	assert.Equal(t, "4", block.Options[1].Text)
	assert.True(t, block.Options[1].IsCorrect)
}

func TestFromYAML_Quiz_NoOptions(t *testing.T) {
	fields := map[string]any{
		"question": "No options yet",
	}
	data, err := blocks.FromYAML("quiz", fields)
	require.NoError(t, err)

	var block blocks.QuizBlock
	block.BaseBlock.Data = data
	require.NoError(t, block.ParseData())
	assert.Equal(t, "No options yet", block.Question)
	assert.Empty(t, block.Options)
}

func TestFromYAML_Quiz_PreservesOptionalFields(t *testing.T) {
	fields := map[string]any{
		"question":        "Pick all that apply",
		"multiple_choice": true,
		"randomise_order": true,
		"allow_retry":     true,
		"unlocked_content": "Well done!",
		"options": []any{
			map[string]any{"text": "A", "correct": true},
		},
	}

	data, err := blocks.FromYAML("quiz", fields)
	require.NoError(t, err)

	var block blocks.QuizBlock
	block.BaseBlock.Data = data
	require.NoError(t, block.ParseData())

	assert.True(t, block.MultipleChoice)
	assert.True(t, block.RandomizeOrder)
	assert.True(t, block.RetryEnabled)
	assert.Equal(t, "Well done!", block.UnlockedContent)
}

// --- checklistFromYAML ---

func TestFromYAML_Checklist_StringItemsGetIDs(t *testing.T) {
	fields := map[string]any{
		"items": []any{"Do thing A", "Do thing B", "Do thing C"},
	}

	data, err := blocks.FromYAML("checklist", fields)
	require.NoError(t, err)

	var block blocks.ChecklistBlock
	block.BaseBlock.Data = data
	require.NoError(t, block.ParseData())

	require.Len(t, block.List, 3)
	assert.Equal(t, "Do thing A", block.List[0].Description)
	assert.Equal(t, "Do thing B", block.List[1].Description)
	assert.Equal(t, "Do thing C", block.List[2].Description)

	for i, item := range block.List {
		assert.NotEmpty(t, item.ID, "item %d should have a generated ID", i)
		assert.False(t, item.Checked, "item %d should default to unchecked", i)
	}

	ids := []string{block.List[0].ID, block.List[1].ID, block.List[2].ID}
	assert.Equal(t, 3, len(uniqueStrings(ids)), "all IDs must be unique")
}

func TestFromYAML_Checklist_ObjectItemReturnsError(t *testing.T) {
	fields := map[string]any{
		"items": []any{
			map[string]any{"description": "I am an object, not a string"},
		},
	}

	_, err := blocks.FromYAML("checklist", fields)
	require.Error(t, err)
	assert.ErrorIs(t, err, blocks.ErrInvalidItemFormat)
}

// --- sortingFromYAML ---

func TestFromYAML_Sorting_StringItemsGetIDsAndPositions(t *testing.T) {
	fields := map[string]any{
		"items": []any{"First", "Second", "Third"},
	}

	data, err := blocks.FromYAML("sorting", fields)
	require.NoError(t, err)

	var block blocks.SortingBlock
	block.BaseBlock.Data = data
	require.NoError(t, block.ParseData())

	require.Len(t, block.Items, 3)
	assert.Equal(t, "First", block.Items[0].Description)
	assert.Equal(t, "Second", block.Items[1].Description)
	assert.Equal(t, "Third", block.Items[2].Description)

	assert.Equal(t, 1, block.Items[0].Position)
	assert.Equal(t, 2, block.Items[1].Position)
	assert.Equal(t, 3, block.Items[2].Position)

	for i, item := range block.Items {
		assert.NotEmpty(t, item.ID, "item %d should have a generated ID", i)
	}

	ids := []string{block.Items[0].ID, block.Items[1].ID, block.Items[2].ID}
	assert.Equal(t, 3, len(uniqueStrings(ids)), "all IDs must be unique")
}

func TestFromYAML_Sorting_ObjectItemReturnsError(t *testing.T) {
	fields := map[string]any{
		"items": []any{
			map[string]any{"description": "I am an object, not a string", "position": 1},
		},
	}

	_, err := blocks.FromYAML("sorting", fields)
	require.Error(t, err)
	assert.ErrorIs(t, err, blocks.ErrInvalidItemFormat)
}

// --- Round-trip: ToYAML → FromYAML → ParseData ---

func TestRoundTrip_Quiz(t *testing.T) {
	original := blocks.QuizBlock{
		Question:       "What colour is the sky?",
		MultipleChoice: false,
		RandomizeOrder: true,
		RetryEnabled:   false,
		UnlockedContent: "Correct!",
		Options: []blocks.QuizOption{
			{ID: "opt-1", Text: "Blue", IsCorrect: true, Order: 0},
			{ID: "opt-2", Text: "Green", IsCorrect: false, Order: 1},
		},
	}

	yamlFields := original.ToYAML()
	data, err := blocks.FromYAML("quiz", yamlFields)
	require.NoError(t, err)

	var restored blocks.QuizBlock
	restored.BaseBlock.Data = data
	require.NoError(t, restored.ParseData())

	assert.Equal(t, original.Question, restored.Question)
	assert.Equal(t, original.MultipleChoice, restored.MultipleChoice)
	assert.Equal(t, original.RandomizeOrder, restored.RandomizeOrder)
	assert.Equal(t, original.RetryEnabled, restored.RetryEnabled)
	assert.Equal(t, original.UnlockedContent, restored.UnlockedContent)
	require.Len(t, restored.Options, 2)
	assert.Equal(t, "Blue", restored.Options[0].Text)
	assert.True(t, restored.Options[0].IsCorrect)
	assert.Equal(t, "Green", restored.Options[1].Text)
	assert.False(t, restored.Options[1].IsCorrect)
	// IDs are regenerated on import — just check they're present
	assert.NotEmpty(t, restored.Options[0].ID)
	assert.NotEmpty(t, restored.Options[1].ID)
}

func TestRoundTrip_Checklist(t *testing.T) {
	original := blocks.ChecklistBlock{
		Content: "Complete all tasks",
		List: []blocks.ChecklistItem{
			{ID: "c1", Description: "Task one", Checked: false},
			{ID: "c2", Description: "Task two", Checked: false},
		},
	}

	yamlFields := original.ToYAML()
	data, err := blocks.FromYAML("checklist", yamlFields)
	require.NoError(t, err)

	var restored blocks.ChecklistBlock
	restored.BaseBlock.Data = data
	require.NoError(t, restored.ParseData())

	assert.Equal(t, original.Content, restored.Content)
	require.Len(t, restored.List, 2)
	assert.Equal(t, "Task one", restored.List[0].Description)
	assert.Equal(t, "Task two", restored.List[1].Description)
	assert.False(t, restored.List[0].Checked)
	assert.NotEmpty(t, restored.List[0].ID)
}

func TestRoundTrip_Sorting(t *testing.T) {
	original := blocks.SortingBlock{
		Content:       "Order these steps",
		ScoringScheme: blocks.AllOrNothing,
		Items: []blocks.SortingItem{
			{ID: "s1", Description: "Step A", Position: 1},
			{ID: "s2", Description: "Step B", Position: 2},
			{ID: "s3", Description: "Step C", Position: 3},
		},
	}

	yamlFields := original.ToYAML()
	data, err := blocks.FromYAML("sorting", yamlFields)
	require.NoError(t, err)

	var restored blocks.SortingBlock
	restored.BaseBlock.Data = data
	require.NoError(t, restored.ParseData())

	assert.Equal(t, original.Content, restored.Content)
	assert.Equal(t, original.ScoringScheme, restored.ScoringScheme)
	require.Len(t, restored.Items, 3)
	assert.Equal(t, "Step A", restored.Items[0].Description)
	assert.Equal(t, 1, restored.Items[0].Position)
	assert.Equal(t, "Step B", restored.Items[1].Description)
	assert.Equal(t, 2, restored.Items[1].Position)
}

// uniqueStrings returns the set of distinct values in s.
func uniqueStrings(s []string) map[string]struct{} {
	m := make(map[string]struct{}, len(s))
	for _, v := range s {
		m[v] = struct{}{}
	}
	return m
}
