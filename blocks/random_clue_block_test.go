package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomClueBlock_ParseData(t *testing.T) {
	clues := []string{"First clue", "Second clue", "Third clue"}
	data, err := json.Marshal(map[string]interface{}{
		"clues": clues,
	})
	require.NoError(t, err)

	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err = block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, clues, block.Clues)
}

func TestRandomClueBlock_UpdateBlockData(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string][]string
		expected []string
	}{
		{
			name: "update with multiple clues",
			input: map[string][]string{
				"clues": {"First clue", "Second clue", "Third clue"},
			},
			expected: []string{"First clue", "Second clue", "Third clue"},
		},
		{
			name: "filter out empty clues",
			input: map[string][]string{
				"clues": {"Valid clue", "", "Another clue", "   ", "Final clue"},
			},
			expected: []string{"Valid clue", "Another clue", "   ", "Final clue"},
		},
		{
			name: "handle empty input",
			input: map[string][]string{
				"clues": {},
			},
			expected: []string(nil),
		},
		{
			name: "handle all empty clues",
			input: map[string][]string{
				"clues": {"", "   ", ""},
			},
			expected: []string{"   "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.RandomClueBlock{}
			err := block.UpdateBlockData(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, block.Clues)
		})
	}
}

func TestRandomClueBlock_GetClue(t *testing.T) {
	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block-id",
		},
		Clues: []string{
			"First clue",
			"Second clue",
			"Third clue",
			"Fourth clue",
		},
	}

	// Test deterministic selection - same team should always get same clue
	team1Clue1 := block.GetClue("team1")
	team1Clue2 := block.GetClue("team1")
	assert.Equal(t, team1Clue1, team1Clue2, "Same team should get same clue consistently")

	// Test different teams get different clues (statistically likely)
	team2Clue := block.GetClue("team2")
	team3Clue := block.GetClue("team3")

	// While not guaranteed, it's extremely unlikely all teams get the same clue
	clues := []string{team1Clue1, team2Clue, team3Clue}
	uniqueClues := make(map[string]bool)
	for _, clue := range clues {
		uniqueClues[clue] = true
	}

	// With 4 clues and 3 teams, we should get some variety
	assert.Greater(t, len(uniqueClues), 1, "Different teams should likely get different clues")

	// All returned clues should be from our clue list
	for _, clue := range clues {
		assert.Contains(t, block.Clues, clue, "Returned clue should be from the clues list")
	}
}

func TestRandomClueBlock_GetClue_EmptyClues(t *testing.T) {
	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block-id",
		},
		Clues: []string{},
	}

	clue := block.GetClue("any-team")
	assert.Equal(t, "No clues available", clue)
}

func TestRandomClueBlock_GetClue_SingleClue(t *testing.T) {
	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{
			ID: "test-block-id",
		},
		Clues: []string{"Only clue"},
	}

	// All teams should get the only available clue
	assert.Equal(t, "Only clue", block.GetClue("team1"))
	assert.Equal(t, "Only clue", block.GetClue("team2"))
	assert.Equal(t, "Only clue", block.GetClue("team3"))
}

func TestRandomClueBlock_GetClue_Deterministic(t *testing.T) {
	// Test that the same team + block combination always produces the same result
	block1 := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{ID: "block-1"},
		Clues:     []string{"A", "B", "C", "D", "E"},
	}

	block2 := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{ID: "block-2"},
		Clues:     []string{"A", "B", "C", "D", "E"},
	}

	// Same team with same block should be consistent
	team1Block1_1 := block1.GetClue("team1")
	team1Block1_2 := block1.GetClue("team1")
	assert.Equal(t, team1Block1_1, team1Block1_2)

	// Same team with different blocks should potentially be different
	team1Block2 := block2.GetClue("team1")
	// Note: Could be the same by chance, but algorithm should be different

	// Different teams with same block should potentially be different
	team2Block1 := block1.GetClue("team2")
	// Note: Could be the same by chance, but algorithm should distribute

	// Verify all results are valid clues
	validClues := map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true}
	assert.True(t, validClues[team1Block1_1])
	assert.True(t, validClues[team1Block1_2])
	assert.True(t, validClues[team1Block2])
	assert.True(t, validClues[team2Block1])
}

func TestRandomClueBlock_RequiresValidation(t *testing.T) {
	block := blocks.RandomClueBlock{}
	assert.False(t, block.RequiresValidation(), "Random clue block should not require validation")
}

func TestRandomClueBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 0,
		},
		Clues: []string{"Test clue"},
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
	assert.True(t, newState.IsComplete(), "Random clue block should auto-complete")
	assert.Equal(t, 0, newState.GetPointsAwarded(), "Random clue block should not award points")

	// Test with empty input
	emptyInput := map[string][]string{}
	newState2, err := block.ValidatePlayerInput(state, emptyInput)
	require.NoError(t, err)
	assert.True(t, newState2.IsComplete(), "Random clue block should auto-complete even with empty input")
}

func TestRandomClueBlock_GetData(t *testing.T) {
	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "test-id",
			Points: 0,
		},
		Clues: []string{"First clue", "Second clue"},
	}

	data := block.GetData()
	assert.NotNil(t, data)

	// Verify we can unmarshal the data
	var unmarshaled blocks.RandomClueBlock
	err := json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, []string{"First clue", "Second clue"}, unmarshaled.Clues)
}

func TestRandomClueBlock_Distribution(t *testing.T) {
	// Test that the distribution across teams is reasonably even
	block := blocks.RandomClueBlock{
		BaseBlock: blocks.BaseBlock{ID: "test-block"},
		Clues:     []string{"A", "B", "C", "D"},
	}

	// Generate results for many teams
	results := make(map[string]int)
	numTeams := 100

	for i := range numTeams {
		teamCode := "team" + string(rune(i))
		clue := block.GetClue(teamCode)
		results[clue]++
	}

	// Each clue should be selected at least a few times
	// With 100 teams and 4 clues, we expect roughly 25 per clue
	// Allow for some variation but ensure no clue is completely ignored
	for clue, count := range results {
		assert.Greater(t, count, 5, "Clue '%s' should be selected multiple times", clue)
		assert.Less(t, count, 70, "Clue '%s' should not dominate distribution", clue)
	}

	// All 4 clues should be represented
	assert.Len(t, results, 4, "All clues should be selected at least once")
}
