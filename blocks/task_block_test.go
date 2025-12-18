package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskBlock_Getters(t *testing.T) {
	block := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-task-id",
			LocationID: "location-123",
			Order:      2,
			Points:     100,
		},
		TaskName: "Find the hidden treasure",
	}

	assert.Equal(t, "task", block.GetType()) // Should never change
	assert.Equal(t, "test-task-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 2, block.GetOrder())
	assert.Equal(t, 100, block.GetPoints())
	// Icon should be default since no inner block
	assert.Contains(t, block.GetIconSVG(), "svg")
}

func TestTaskBlock_GetIconSVG_AutoDerive(t *testing.T) {
	// Task block without custom icon should derive from inner
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{Points: 50},
		TaskName:  "Take a photo",
		InnerType: "answer", // Start with a different type
	}

	// Step 1: Change to photo type
	err := task.UpdateBlockData(map[string][]string{
		"task_name":  {"Take a photo"},
		"inner_type": {"photo"},
	})
	require.NoError(t, err)

	// Step 2: Initialize photo block with required fields
	err = task.UpdateBlockData(map[string][]string{
		"prompt": {"Take a photo of the landmark"},
	})
	require.NoError(t, err)

	// Icon should contain svg (default or from photo block)
	assert.Contains(t, task.GetIconSVG(), "svg")
}

func TestTaskBlock_GetIconSVG_Default(t *testing.T) {
	// Task block without custom icon or inner block should use default
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{Points: 50},
		TaskName:  "Complete task",
	}

	icon := task.GetIconSVG()
	assert.Contains(t, icon, "svg")
	assert.Contains(t, icon, "xmlns")
}

func TestTaskBlock_ParseData_WithPhotoBlock(t *testing.T) {
	// Create inner photo block data
	photoData := map[string]any{
		"prompt":     "Take a photo of the landmark",
		"max_images": 1,
	}
	photoJSON, err := json.Marshal(photoData)
	require.NoError(t, err)

	// Create task block data
	taskData := map[string]any{
		"task_name":  "Photo Challenge",
		"task_icon":  "",
		"inner_type": "photo",
		"inner_data": json.RawMessage(photoJSON),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	// Create task block with data
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "task-1",
			LocationID: "loc-1",
			Type:       "task",
			Data:       json.RawMessage(taskJSON),
			Points:     100,
		},
	}

	err = task.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Photo Challenge", task.TaskName)
	assert.Equal(t, "photo", task.InnerType)
	assert.NotNil(t, task.GetInnerBlock())
	assert.Equal(t, "photo", task.GetInnerBlock().GetType())
}

func TestTaskBlock_ParseData_WithQuizBlock(t *testing.T) {
	// Create inner quiz block data
	quizData := map[string]any{
		"question": "What is 2+2?",
		"options": []map[string]any{
			{"id": "opt_0", "text": "3", "is_correct": false, "order": 0},
			{"id": "opt_1", "text": "4", "is_correct": true, "order": 1},
		},
		"multiple_choice": false,
		"randomize_order": false,
		"retry_enabled":   false,
	}
	quizJSON, err := json.Marshal(quizData)
	require.NoError(t, err)

	// Create task block data
	taskData := map[string]any{
		"task_name":  "Math Quiz",
		"inner_type": "quiz_block",
		"inner_data": json.RawMessage(quizJSON),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "task-2",
			LocationID: "loc-1",
			Type:       "task",
			Data:       json.RawMessage(taskJSON),
			Points:     50,
		},
	}

	err = task.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Math Quiz", task.TaskName)
	assert.Equal(t, "quiz_block", task.InnerType)
	assert.NotNil(t, task.GetInnerBlock())
	assert.Equal(t, "quiz_block", task.GetInnerBlock().GetType())
}

func TestTaskBlock_ParseData_InvalidInnerType(t *testing.T) {
	taskData := map[string]any{
		"task_name":  "Invalid Task",
		"inner_type": "nonexistent_block",
		"inner_data": json.RawMessage("{}"),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			Data: json.RawMessage(taskJSON),
		},
	}

	err = task.ParseData()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating inner block")
}

func TestTaskBlock_RequiresValidation(t *testing.T) {
	task := blocks.TaskBlock{}
	assert.True(t, task.RequiresValidation())
}

func TestTaskBlock_ValidatePlayerInput_DelegatesToInner(t *testing.T) {
	// Create an answer block as inner validation
	answerData := map[string]any{
		"prompt": "Enter the code",
		"answer": "SECRET",
		"fuzzy":  false,
	}
	answerJSON, err := json.Marshal(answerData)
	require.NoError(t, err)

	taskData := map[string]any{
		"task_name":  "Secret Code",
		"inner_type": "answer",
		"inner_data": json.RawMessage(answerJSON),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "task-3",
			Type:   "task",
			Data:   json.RawMessage(taskJSON),
			Points: 75,
		},
	}

	err = task.ParseData()
	require.NoError(t, err)

	state := &blocks.MockPlayerState{
		BlockID:  "task-3",
		PlayerID: "player-1",
	}

	// Test correct answer
	input := map[string][]string{
		"answer": {"SECRET"},
	}

	newState, err := task.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 75, newState.GetPointsAwarded()) // Task's points, not inner block's
}

func TestTaskBlock_ValidatePlayerInput_IncorrectAnswer(t *testing.T) {
	// Create an answer block as inner validation
	answerData := map[string]any{
		"prompt": "Enter the code",
		"answer": "SECRET",
		"fuzzy":  false,
	}
	answerJSON, err := json.Marshal(answerData)
	require.NoError(t, err)

	taskData := map[string]any{
		"task_name":  "Secret Code",
		"inner_type": "answer",
		"inner_data": json.RawMessage(answerJSON),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "task-4",
			Type:   "task",
			Data:   json.RawMessage(taskJSON),
			Points: 75,
		},
	}

	err = task.ParseData()
	require.NoError(t, err)

	state := &blocks.MockPlayerState{
		BlockID:  "task-4",
		PlayerID: "player-1",
	}

	// Test incorrect answer - PasswordBlock returns no error on wrong answer
	input := map[string][]string{
		"answer": {"WRONG"},
	}

	newState, err := task.ValidatePlayerInput(state, input)
	require.NoError(t, err) // No error on wrong answer, just incomplete
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())
}

func TestTaskBlock_ValidatePlayerInput_NoInnerBlock(t *testing.T) {
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "task-5",
			Points: 50,
		},
		TaskName: "Incomplete Task",
	}

	state := &blocks.MockPlayerState{
		BlockID:  "task-5",
		PlayerID: "player-1",
	}

	input := map[string][]string{}

	newState, err := task.ValidatePlayerInput(state, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not properly configured")
	assert.False(t, newState.IsComplete())
}

func TestTaskBlock_UpdateBlockData_TaskFields(t *testing.T) {
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{},
	}

	input := map[string][]string{
		"task_name": {"Updated Task Name"},
		"points":    {"150"},
	}

	err := task.UpdateBlockData(input)
	require.NoError(t, err)
	assert.Equal(t, "Updated Task Name", task.TaskName)
	assert.Equal(t, 150, task.Points)
}

func TestTaskBlock_UpdateBlockData_TaskNameValidation(t *testing.T) {
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{},
	}

	// Test task name that's too long
	longName := string(make([]byte, 201))
	for i := range longName {
		longName = string(append([]byte(longName[:i]), 'a'))
	}

	input := map[string][]string{
		"task_name": {longName},
	}

	err := task.UpdateBlockData(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

func TestTaskBlock_UpdateBlockData_ChangeInnerType(t *testing.T) {
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{},
		InnerType: "photo",
	}

	// Step 1: Change inner type (fields ignored when type changes)
	input1 := map[string][]string{
		"inner_type": {"answer"},
		"prompt":     {"This will be ignored"},
		"answer":     {"This will be ignored"},
	}

	err := task.UpdateBlockData(input1)
	require.NoError(t, err)
	assert.Equal(t, "answer", task.InnerType)
	assert.NotNil(t, task.GetInnerBlock())
	assert.Equal(t, "answer", task.GetInnerBlock().GetType())

	// Step 2: Update the new answer block with proper fields
	input2 := map[string][]string{
		"prompt": {"Enter the answer"},
		"answer": {"TEST"},
	}

	err = task.UpdateBlockData(input2)
	require.NoError(t, err)
}

func TestTaskBlock_UpdateBlockData_DelegateToInner(t *testing.T) {
	// Create task starting with answer block
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{},
		InnerType: "answer",
	}

	// Step 1: Change to photo inner block (fields ignored during type change)
	input1 := map[string][]string{
		"inner_type": {"photo"},
		"prompt":     {"This will be ignored"},
	}
	err := task.UpdateBlockData(input1)
	require.NoError(t, err)
	assert.NotNil(t, task.GetInnerBlock())
	assert.Equal(t, "photo", task.GetInnerBlock().GetType())

	// Step 2: Initialize photo block with required fields
	input2 := map[string][]string{
		"prompt": {"Take a photo"},
	}
	err = task.UpdateBlockData(input2)
	require.NoError(t, err)

	// Step 3: Update inner block fields (photo still active, type not changing)
	input3 := map[string][]string{
		"prompt":     {"New prompt for photo"},
		"max_images": {"3"},
	}

	err = task.UpdateBlockData(input3)
	require.NoError(t, err)

	// Verify inner block data was updated
	assert.NotNil(t, task.InnerData)
}

func TestTaskBlock_GetInnerBlock(t *testing.T) {
	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{},
	}

	// Initially nil
	assert.Nil(t, task.GetInnerBlock())

	// After parsing, should return inner block
	photoData := map[string]any{
		"prompt": "Take a photo",
	}
	photoJSON, err := json.Marshal(photoData)
	require.NoError(t, err)

	taskData := map[string]any{
		"task_name":  "Photo Task",
		"inner_type": "photo",
		"inner_data": json.RawMessage(photoJSON),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	task.Data = json.RawMessage(taskJSON)
	err = task.ParseData()
	require.NoError(t, err)

	innerBlock := task.GetInnerBlock()
	assert.NotNil(t, innerBlock)
	assert.Equal(t, "photo", innerBlock.GetType())
}

func TestGetTaskValidationTypes(t *testing.T) {
	types := blocks.GetTaskValidationTypes()

	// Should return validation-capable blocks
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "photo")
	assert.Contains(t, types, "quiz_block")
	assert.Contains(t, types, "answer")
	assert.Contains(t, types, "pincode")
	assert.Contains(t, types, "sorting")
	assert.Contains(t, types, "checklist")

	// Should not contain non-validation blocks
	assert.NotContains(t, types, "markdown")
	assert.NotContains(t, types, "divider")
	assert.NotContains(t, types, "alert")
}

func TestNewTaskBlock(t *testing.T) {
	base := blocks.BaseBlock{
		ID:         "test-id",
		LocationID: "location-123",
		Type:       "task",
		Order:      1,
		Points:     100,
	}

	task := blocks.NewTaskBlock(base)

	assert.Equal(t, base, task.BaseBlock)
	assert.Empty(t, task.TaskName)
	assert.Equal(t, "qr_code", task.InnerType) // Default inner type
	assert.Nil(t, task.GetInnerBlock())        // Not parsed yet
}

func TestTaskBlock_PointsLiveOnTask_NotInner(t *testing.T) {
	// Create a quiz block with its own points
	quizData := map[string]any{
		"question": "What is 2+2?",
		"options": []map[string]any{
			{"id": "opt_0", "text": "4", "is_correct": true, "order": 0},
		},
		"multiple_choice": false,
	}
	quizJSON, err := json.Marshal(quizData)
	require.NoError(t, err)

	taskData := map[string]any{
		"task_name":  "Math Challenge",
		"inner_type": "quiz_block",
		"inner_data": json.RawMessage(quizJSON),
	}
	taskJSON, err := json.Marshal(taskData)
	require.NoError(t, err)

	task := blocks.TaskBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "task-6",
			Type:   "task",
			Data:   json.RawMessage(taskJSON),
			Points: 200, // Task has 200 points
		},
	}

	err = task.ParseData()
	require.NoError(t, err)

	// Task's GetPoints should return task points
	assert.Equal(t, 200, task.GetPoints())

	// Inner block should have task's points (passed through during ParseData)
	assert.Equal(t, 200, task.GetInnerBlock().GetPoints())

	// Validation delegated to inner block which awards its points
	state := &blocks.MockPlayerState{
		BlockID:  "task-6",
		PlayerID: "player-1",
	}

	input := map[string][]string{
		"quiz_option": {"opt_0"}, // Correct answer
	}

	newState, err := task.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, 200, newState.GetPointsAwarded()) // Inner block awards task's points
}
