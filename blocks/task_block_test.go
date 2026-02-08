package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskBlock_GetName(t *testing.T) {
	block := blocks.TaskBlock{}
	// Ensure GetName always returns "Task" and never changes
	assert.Equal(t, "Task", block.GetName())
}

func TestTaskBlock_Getters(t *testing.T) {
	block := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Task:        "Take a photo of the landmark",
		Icon:        "camera",
		LinkThrough: true,
	}

	assert.Equal(t, "Task", block.GetName())
	assert.Equal(t, "Give players a task to complete.", block.GetDescription())
	assert.Equal(t, "task", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
	assert.NotEmpty(t, block.GetIconSVG())
}

func TestTaskBlock_ParseData(t *testing.T) {
	data := `{"task":"Find the hidden treasure","icon":"map","link_through":true}`
	block := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Find the hidden treasure", block.Task)
	assert.Equal(t, "map", block.Icon)
	assert.True(t, block.LinkThrough)
}

func TestTaskBlock_ParseData_MinimalFields(t *testing.T) {
	data := `{"task":"Complete the quiz"}`
	block := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Complete the quiz", block.Task)
	assert.Empty(t, block.Icon)
	assert.False(t, block.LinkThrough)
}

func TestTaskBlock_UpdateBlockData(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string][]string
		wantTask    string
		wantIcon    string
		wantLink    bool
		wantErr     bool
		errContains string
	}{
		{
			name: "all fields populated",
			input: map[string][]string{
				"task":         {"Take a photo"},
				"icon":         {"camera"},
				"link_through": {"on"},
			},
			wantTask: "Take a photo",
			wantIcon: "camera",
			wantLink: true,
			wantErr:  false,
		},
		{
			name: "link_through with value 1",
			input: map[string][]string{
				"task":         {"Record audio"},
				"icon":         {"mic"},
				"link_through": {"1"},
			},
			wantTask: "Record audio",
			wantIcon: "mic",
			wantLink: true,
			wantErr:  false,
		},
		{
			name: "minimal fields - only task",
			input: map[string][]string{
				"task": {"Answer the question"},
			},
			wantTask: "Answer the question",
			wantIcon: "",
			wantLink: false,
			wantErr:  false,
		},
		{
			name: "task with icon but no link_through",
			input: map[string][]string{
				"task": {"Visit the location"},
				"icon": {"map"},
			},
			wantTask: "Visit the location",
			wantIcon: "map",
			wantLink: false,
			wantErr:  false,
		},
		{
			name: "link_through unchecked",
			input: map[string][]string{
				"task":         {"Complete challenge"},
				"icon":         {"dices"},
				"link_through": {"off"},
			},
			wantTask: "Complete challenge",
			wantIcon: "dices",
			wantLink: false,
			wantErr:  false,
		},
		{
			name:        "missing task field",
			input:       map[string][]string{},
			wantErr:     true,
			errContains: "task is a required field",
		},
		{
			name: "empty task field",
			input: map[string][]string{
				"task": {},
			},
			wantErr:     true,
			errContains: "task is a required field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := blocks.TaskBlock{}
			err := block.UpdateBlockData(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantTask, block.Task)
			assert.Equal(t, tt.wantIcon, block.Icon)
			assert.Equal(t, tt.wantLink, block.LinkThrough)
		})
	}
}

func TestTaskBlock_RequiresValidation(t *testing.T) {
	block := blocks.TaskBlock{}
	// Task blocks should never require validation
	assert.False(t, block.RequiresValidation())
}

func TestTaskBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 10,
		},
		Task:        "Take a photo of the statue",
		Icon:        "camera",
		LinkThrough: true,
	}

	state := &blocks.MockPlayerState{}

	// Task blocks should mark as complete without any input validation
	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)

	assert.True(t, newState.IsComplete())
	// Task blocks don't award points directly (handled by checkin completion)
	assert.Equal(t, 0, newState.GetPointsAwarded())
}
