package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// choiceState builds a MockPlayerState with the given chosen vars pre-loaded.
func choiceState(chosen ...string) *blocks.MockPlayerState {
	s := &blocks.MockPlayerState{}
	data, _ := json.Marshal(struct {
		Chosen []string `json:"chosen"`
	}{chosen})
	s.SetPlayerData(data)
	return s
}

func TestChoiceBlock_Getters(t *testing.T) {
	block := blocks.ChoiceBlock{
		BaseBlock: blocks.BaseBlock{
			ID:      "choice-id",
			OwnerID: "location-abc",
			Order:   2,
			Points:  25,
		},
		Prompt: "Which way?",
		Options: []blocks.ChoiceOption{
			{Label: "Left", Sets: "went_left"},
			{Label: "Right", Sets: "went_right"},
		},
	}

	assert.Equal(t, "choice", block.GetType())
	assert.Equal(t, "choice-id", block.GetID())
	assert.Equal(t, "location-abc", block.GetOwnerID())
	assert.Equal(t, 2, block.GetOrder())
	assert.Equal(t, 25, block.GetPoints())
	assert.Contains(t, block.GetIconSVG(), "svg")
	assert.True(t, block.RequiresValidation())
	assert.True(t, block.SupportsVariableSets())
}

func TestChoiceBlock_ParseData(t *testing.T) {
	data := `{
		"prompt": "Choose your path",
		"button_text": "Make your choice",
		"multi_select": true,
		"options": [
			{"label": "Forest", "sets": "forest"},
			{"label": "Mountain", "sets": "mountain"}
		]
	}`

	block := blocks.ChoiceBlock{
		BaseBlock: blocks.BaseBlock{Data: json.RawMessage(data)},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Choose your path", block.Prompt)
	assert.Equal(t, "Make your choice", block.ButtonText)
	assert.True(t, block.MultiSelect)
	require.Len(t, block.Options, 2)
	assert.Equal(t, "Forest", block.Options[0].Label)
	assert.Equal(t, "forest", block.Options[0].Sets)
}

func TestChoiceBlock_UpdateBlockData(t *testing.T) {
	t.Run("full update single-select", func(t *testing.T) {
		block := blocks.ChoiceBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"prompt":       {"Which path?"},
			"button_text":  {"Go!"},
			"points":       {"10"},
			"option_label": {"Left", "Right"},
			"option_sets":  {"went_left", "went_right"},
		})
		require.NoError(t, err)
		assert.Equal(t, "Which path?", block.Prompt)
		assert.Equal(t, "Go!", block.ButtonText)
		assert.Equal(t, 10, block.Points)
		assert.False(t, block.MultiSelect)
		require.Len(t, block.Options, 2)
		assert.Equal(t, blocks.ChoiceOption{Label: "Left", Sets: "went_left"}, block.Options[0])
	})

	t.Run("multi_select toggle on", func(t *testing.T) {
		block := blocks.ChoiceBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"multi_select": {"on"},
			"option_label": {"A"},
			"option_sets":  {"a"},
		})
		require.NoError(t, err)
		assert.True(t, block.MultiSelect)
	})

	t.Run("multi_select absent = false", func(t *testing.T) {
		block := blocks.ChoiceBlock{MultiSelect: true}
		err := block.UpdateBlockData(map[string][]string{"prompt": {"X"}})
		require.NoError(t, err)
		assert.False(t, block.MultiSelect)
	})

	t.Run("empty rows filtered", func(t *testing.T) {
		block := blocks.ChoiceBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"option_label": {"Valid", "", "Also valid", ""},
			"option_sets":  {"valid", "", "also_valid", ""},
		})
		require.NoError(t, err)
		require.Len(t, block.Options, 2)
		assert.Equal(t, "Valid", block.Options[0].Label)
		assert.Equal(t, "Also valid", block.Options[1].Label)
	})

	t.Run("invalid points", func(t *testing.T) {
		block := blocks.ChoiceBlock{}
		err := block.UpdateBlockData(map[string][]string{"points": {"not-a-number"}})
		assert.Error(t, err)
	})
}

func TestChoiceBlock_GetButtonText(t *testing.T) {
	block := blocks.ChoiceBlock{}
	assert.Equal(t, "Confirm choice", block.GetButtonText())

	block.ButtonText = "Go!"
	assert.Equal(t, "Go!", block.GetButtonText())
}

func TestChoiceBlock_GetSets(t *testing.T) {
	block := blocks.ChoiceBlock{
		Options: []blocks.ChoiceOption{
			{Label: "Left", Sets: "went_left"},
			{Label: "Right", Sets: "went_right"},
			{Label: "Empty", Sets: ""},
		},
	}
	sets := block.GetSets()
	assert.Equal(t, []string{"went_left", "went_right"}, sets)
}

func TestChoiceBlock_ValidatePlayerInput_SingleSelect(t *testing.T) {
	block := blocks.ChoiceBlock{
		BaseBlock: blocks.BaseBlock{Points: 20},
		Options: []blocks.ChoiceOption{
			{Label: "Forest", Sets: "forest"},
			{Label: "Mountain", Sets: "mountain"},
		},
	}

	t.Run("valid choice", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		newState, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"forest"},
		})
		require.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, 20, newState.GetPointsAwarded())
		assert.Equal(t, []string{"forest"}, block.GetChosenVars(newState))
	})

	t.Run("only first value accepted even if multiple submitted", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		newState, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"forest", "mountain"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"forest"}, block.GetChosenVars(newState))
	})

	t.Run("invalid choice rejected", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		_, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"invalid_var"},
		})
		assert.Error(t, err)
	})

	t.Run("no choice submitted", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		_, err := block.ValidatePlayerInput(state, map[string][]string{})
		assert.Error(t, err)
	})

	t.Run("already complete is no-op", func(t *testing.T) {
		completedState, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, map[string][]string{"choice": {"forest"}})
		require.NoError(t, err)
		require.True(t, completedState.IsComplete())

		newState, err := block.ValidatePlayerInput(completedState, map[string][]string{"choice": {"mountain"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"forest"}, block.GetChosenVars(newState)) // unchanged
	})
}

func TestChoiceBlock_ValidatePlayerInput_MultiSelect(t *testing.T) {
	block := blocks.ChoiceBlock{
		BaseBlock:   blocks.BaseBlock{Points: 15},
		MultiSelect: true,
		Options: []blocks.ChoiceOption{
			{Label: "Forest", Sets: "forest"},
			{Label: "Mountain", Sets: "mountain"},
			{Label: "River", Sets: "river"},
		},
	}

	t.Run("single selection", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		newState, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"forest"},
		})
		require.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, []string{"forest"}, block.GetChosenVars(newState))
	})

	t.Run("multiple selections", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		newState, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"forest", "river"},
		})
		require.NoError(t, err)
		assert.True(t, newState.IsComplete())
		assert.Equal(t, []string{"forest", "river"}, block.GetChosenVars(newState))
		assert.Equal(t, 15, newState.GetPointsAwarded())
	})

	t.Run("duplicates deduplicated", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		newState, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"mountain", "mountain", "forest"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"mountain", "forest"}, block.GetChosenVars(newState))
	})

	t.Run("invalid option rejected", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		_, err := block.ValidatePlayerInput(state, map[string][]string{
			"choice": {"forest", "bad_var"},
		})
		assert.Error(t, err)
	})

	t.Run("no selection rejected", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		_, err := block.ValidatePlayerInput(state, map[string][]string{})
		assert.Error(t, err)
	})
}

func TestChoiceBlock_GetTriggeredVars(t *testing.T) {
	block := blocks.ChoiceBlock{
		Options: []blocks.ChoiceOption{
			{Label: "Forest", Sets: "forest"},
			{Label: "Mountain", Sets: "mountain"},
		},
	}

	t.Run("incomplete returns nil", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		assert.Nil(t, block.GetTriggeredVars(state))
	})

	t.Run("single chosen var set to true", func(t *testing.T) {
		state := choiceState("mountain")
		state.SetComplete(true)

		vars := block.GetTriggeredVars(state)
		require.NotNil(t, vars)
		assert.Equal(t, "true", vars["mountain"])
		assert.NotContains(t, vars, "forest")
	})

	t.Run("multiple chosen vars all set to true", func(t *testing.T) {
		state := choiceState("forest", "mountain")
		state.SetComplete(true)

		vars := block.GetTriggeredVars(state)
		require.NotNil(t, vars)
		assert.Equal(t, "true", vars["forest"])
		assert.Equal(t, "true", vars["mountain"])
	})

	t.Run("malformed player data returns nil", func(t *testing.T) {
		state := &blocks.MockPlayerState{}
		state.SetPlayerData([]byte("not json"))
		state.SetComplete(true)
		assert.Nil(t, block.GetTriggeredVars(state))
	})
}

func TestChoiceBlock_GetChosenVars(t *testing.T) {
	block := blocks.ChoiceBlock{}

	assert.Nil(t, block.GetChosenVars(nil))
	assert.Nil(t, block.GetChosenVars(&blocks.MockPlayerState{}))

	state := choiceState("forest", "river")
	assert.Equal(t, []string{"forest", "river"}, block.GetChosenVars(state))
}

func TestChoiceBlock_GetChosenLabels(t *testing.T) {
	block := blocks.ChoiceBlock{
		Options: []blocks.ChoiceOption{
			{Label: "Forest path", Sets: "forest"},
			{Label: "Mountain road", Sets: "mountain"},
		},
	}

	t.Run("single known var", func(t *testing.T) {
		state := choiceState("mountain")
		assert.Equal(t, []string{"Mountain road"}, block.GetChosenLabels(state))
	})

	t.Run("multiple known vars", func(t *testing.T) {
		state := choiceState("forest", "mountain")
		assert.Equal(t, []string{"Forest path", "Mountain road"}, block.GetChosenLabels(state))
	})

	t.Run("unknown var omitted from labels", func(t *testing.T) {
		state := choiceState("forest", "unknown_var")
		assert.Equal(t, []string{"Forest path"}, block.GetChosenLabels(state))
	})

	t.Run("GetChosenLabel returns comma-joined", func(t *testing.T) {
		state := choiceState("forest", "mountain")
		assert.Equal(t, "Forest path, Mountain road", block.GetChosenLabel(state))
	})
}

func TestChoiceBlock_ToYAML(t *testing.T) {
	t.Run("minimal — no button_text or multi_select", func(t *testing.T) {
		block := blocks.ChoiceBlock{
			Prompt:  "Choose wisely",
			Options: []blocks.ChoiceOption{{Label: "Option A", Sets: "option_a"}},
		}
		m := block.ToYAML()
		assert.Equal(t, "Choose wisely", m["prompt"])
		assert.NotContains(t, m, "button_text")
		assert.NotContains(t, m, "multi_select")
	})

	t.Run("with button_text", func(t *testing.T) {
		block := blocks.ChoiceBlock{
			ButtonText: "Pick one",
			Options:    []blocks.ChoiceOption{{Label: "A", Sets: "a"}},
		}
		assert.Equal(t, "Pick one", block.ToYAML()["button_text"])
	})

	t.Run("with multi_select", func(t *testing.T) {
		block := blocks.ChoiceBlock{
			MultiSelect: true,
			Options:     []blocks.ChoiceOption{{Label: "A", Sets: "a"}},
		}
		assert.Equal(t, true, block.ToYAML()["multi_select"])
	})
}

func TestChoiceBlock_DocSetsVars(t *testing.T) {
	block := blocks.ChoiceBlock{}

	t.Run("extracts vars from options", func(t *testing.T) {
		doc := game.BlockDoc{
			"options": []any{
				map[string]any{"label": "Forest", "sets": "forest"},
				map[string]any{"label": "Mountain", "sets": "mountain"},
				map[string]any{"label": "No var", "sets": ""},
			},
		}
		vars := block.DocSetsVars(doc)
		assert.ElementsMatch(t, []string{"forest", "mountain"}, vars)
	})

	t.Run("no options returns nil", func(t *testing.T) {
		assert.Nil(t, block.DocSetsVars(game.BlockDoc{}))
	})
}

func TestChoiceBlock_ValidateBlockDoc(t *testing.T) {
	block := blocks.ChoiceBlock{}

	t.Run("valid block", func(t *testing.T) {
		doc := game.BlockDoc{
			"options": []any{
				map[string]any{"label": "Forest", "sets": "forest"},
				map[string]any{"label": "Mountain", "sets": "mountain"},
			},
		}
		errs, warns := block.ValidateBlockDoc("locations[0].blocks[0]", doc)
		assert.Empty(t, errs)
		assert.Empty(t, warns)
	})

	t.Run("no options field", func(t *testing.T) {
		errs, warns := block.ValidateBlockDoc("path", game.BlockDoc{})
		require.Len(t, errs, 1)
		assert.Equal(t, "CHOICE_NO_OPTIONS", errs[0].Code)
		assert.Empty(t, warns)
	})

	t.Run("empty options slice", func(t *testing.T) {
		doc := game.BlockDoc{"options": []any{}}
		errs, _ := block.ValidateBlockDoc("path", doc)
		require.Len(t, errs, 1)
		assert.Equal(t, "CHOICE_NO_OPTIONS", errs[0].Code)
	})

	t.Run("option missing sets", func(t *testing.T) {
		doc := game.BlockDoc{
			"options": []any{
				map[string]any{"label": "Forest", "sets": ""},
			},
		}
		errs, warns := block.ValidateBlockDoc("path", doc)
		require.Len(t, errs, 1)
		assert.Equal(t, "CHOICE_OPTION_MISSING_SETS", errs[0].Code)
		assert.Empty(t, warns)
	})

	t.Run("option missing label", func(t *testing.T) {
		doc := game.BlockDoc{
			"options": []any{
				map[string]any{"label": "", "sets": "forest"},
			},
		}
		errs, warns := block.ValidateBlockDoc("path", doc)
		assert.Empty(t, errs)
		require.Len(t, warns, 1)
		assert.Equal(t, "CHOICE_OPTION_MISSING_LABEL", warns[0].Code)
	})

	t.Run("multiple errors and warnings", func(t *testing.T) {
		doc := game.BlockDoc{
			"options": []any{
				map[string]any{"label": "", "sets": ""},    // missing both
				map[string]any{"label": "OK", "sets": "ok"}, // valid
			},
		}
		errs, warns := block.ValidateBlockDoc("path", doc)
		assert.Len(t, errs, 1)  // missing sets
		assert.Len(t, warns, 1) // missing label
	})
}

func TestNewChoiceBlock(t *testing.T) {
	base := blocks.BaseBlock{
		ID:      "test-id",
		OwnerID: "location-123",
		Type:    "choice",
		Order:   1,
		Points:  50,
	}
	block := blocks.NewChoiceBlock(base)
	assert.Equal(t, base, block.BaseBlock)
	assert.Empty(t, block.Prompt)
	assert.Empty(t, block.Options)
	assert.False(t, block.MultiSelect)
}
