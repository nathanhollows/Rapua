package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomTextBlock_ParseData(t *testing.T) {
	texts := []string{"First text", "Second text", "Third text"}
	data, err := json.Marshal(map[string]interface{}{
		"texts": texts,
	})
	require.NoError(t, err)

	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err = block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, texts, block.Texts)
}

func TestRandomTextBlock_UpdateBlockData(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string][]string
		expected []string
	}{
		{
			name: "update with multiple texts",
			input: map[string][]string{
				"texts": {"First text", "Second text", "Third text"},
			},
			expected: []string{"First text", "Second text", "Third text"},
		},
		{
			name: "filter out empty texts",
			input: map[string][]string{
				"texts": {"Valid text", "", "Another text", "   ", "Final text"},
			},
			expected: []string{"Valid text", "Another text", "   ", "Final text"},
		},
		{
			name: "handle empty input",
			input: map[string][]string{
				"texts": {},
			},
			expected: []string(nil),
		},
		{
			name: "handle all empty texts",
			input: map[string][]string{
				"texts": {"", "   ", ""},
			},
			expected: []string{"   "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.RandomTextBlock{}
			err := block.UpdateBlockData(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, block.Texts)
		})
	}
}

func TestRandomTextBlock_GetText(t *testing.T) {
	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block-id",
		},
		Texts: []string{
			"First text",
			"Second text",
			"Third text",
			"Fourth text",
		},
	}

	// Test deterministic selection - same team should always get same text
	team1Text1 := block.GetText("team1")
	team1Text2 := block.GetText("team1")
	assert.Equal(t, team1Text1, team1Text2, "Same team should get same text consistently")

	// Test different teams get different texts (statistically likely)
	team2Text := block.GetText("team2")
	team3Text := block.GetText("team3")

	// While not guaranteed, it's extremely unlikely all teams get the same text
	texts := []string{team1Text1, team2Text, team3Text}
	uniqueTexts := make(map[string]bool)
	for _, text := range texts {
		uniqueTexts[text] = true
	}

	// With 4 texts and 3 teams, we should get some variety
	assert.Greater(t, len(uniqueTexts), 1, "Different teams should likely get different texts")

	// All returned texts should be from our text list
	for _, text := range texts {
		assert.Contains(t, block.Texts, text, "Returned text should be from the texts list")
	}
}

func TestRandomTextBlock_GetText_EmptyTexts(t *testing.T) {
	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block-id",
		},
		Texts: []string{},
	}

	text := block.GetText("any-team")
	assert.Equal(t, "No text available", text)
}

func TestRandomTextBlock_GetText_SingleText(t *testing.T) {
	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block-id",
		},
		Texts: []string{"Only text"},
	}

	// All teams should get the only available text
	assert.Equal(t, "Only text", block.GetText("team1"))
	assert.Equal(t, "Only text", block.GetText("team2"))
	assert.Equal(t, "Only text", block.GetText("team3"))
}

func TestRandomTextBlock_GetText_Deterministic(t *testing.T) {
	// Test that the same team + block combination always produces the same result
	block1 := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{ID: "block-1"},
		Texts:     []string{"A", "B", "C", "D", "E"},
	}

	block2 := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{ID: "block-2"},
		Texts:     []string{"A", "B", "C", "D", "E"},
	}

	// Same team with same block should be consistent
	team1Block1_1 := block1.GetText("team1")
	team1Block1_2 := block1.GetText("team1")
	assert.Equal(t, team1Block1_1, team1Block1_2)

	// Same team with different blocks should potentially be different
	team1Block2 := block2.GetText("team1")
	// Note: Could be the same by chance, but algorithm should be different

	// Different teams with same block should potentially be different
	team2Block1 := block1.GetText("team2")
	// Note: Could be the same by chance, but algorithm should distribute

	// Verify all results are valid texts
	validTexts := map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true}
	assert.True(t, validTexts[team1Block1_1])
	assert.True(t, validTexts[team1Block1_2])
	assert.True(t, validTexts[team1Block2])
	assert.True(t, validTexts[team2Block1])
}

func TestRandomTextBlock_RequiresValidation(t *testing.T) {
	block := blocks.RandomTextBlock{}
	assert.False(t, block.RequiresValidation(), "Random text block should not require validation")
}

func TestRandomTextBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 0,
		},
		Texts: []string{"Test text"},
	}

	state := &blocks.MockPlayerState{
		//BlockID:  "block-1",
		//PlayerID: "player-1",
	}

	// Any input should mark the block as complete without validation
	input := map[string][]string{
		"any_field": {"any_value"},
	}

	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete(), "Random text block should auto-complete")
	assert.Equal(t, 0, newState.GetPointsAwarded(), "Random text block should not award points")

	// Test with empty input
	emptyInput := map[string][]string{}
	newState2, err := block.ValidatePlayerInput(state, emptyInput)
	require.NoError(t, err)
	assert.True(t, newState2.IsComplete(), "Random text block should auto-complete even with empty input")
}

func TestRandomTextBlock_GetData(t *testing.T) {
	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-id",
			Points: 0,
		},
		Texts: []string{"First text", "Second text"},
	}

	data := block.GetData()
	assert.NotNil(t, data)

	// Verify we can unmarshal the data
	var unmarshaled blocks.RandomTextBlock
	err := json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, []string{"First text", "Second text"}, unmarshaled.Texts)
}

func TestRandomTextBlock_Distribution(t *testing.T) {
	// Test that the distribution across teams is reasonably even
	block := blocks.RandomTextBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Texts:     []string{"A", "B", "C", "D"},
	}

	// Generate results for many teams
	results := make(map[string]int)
	numTeams := 100

	for i := range numTeams {
		teamCode := "run" + string(rune(i))
		text := block.GetText(teamCode)
		results[text]++
	}

	// Each text should be selected at least a few times
	// With 100 teams and 4 texts, we expect roughly 25 per text
	// Allow for some variation but ensure no text is completely ignored
	for text, count := range results {
		assert.Greater(t, count, 5, "Text '%s' should be selected multiple times", text)
		assert.Less(t, count, 70, "Text '%s' should not dominate distribution", text)
	}

	// All 4 texts should be represented
	assert.Len(t, results, 4, "All texts should be selected at least once")
}
