package blocks

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPincodeBlock_Getters(t *testing.T) {
	prompt := gofakeit.Question()
	pincode := strconv.Itoa(gofakeit.Number(1, 999999))
	block := PincodeBlock{
		BaseBlock: BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      1,
			Points:     5,
		},
		Prompt:  prompt,
		Pincode: pincode,
	}

	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
}

func TestPincodeBlock_ParseData(t *testing.T) {
	prompt := gofakeit.Question()
	pincode := strconv.Itoa(gofakeit.Number(1, 999999))
	data := `{"prompt":"` + prompt + `", "pincode":"` + pincode + `"}`
	block := PincodeBlock{
		BaseBlock: BaseBlock{
			Data: []byte(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, prompt, block.Prompt)
	assert.Equal(t, pincode, block.Pincode)
}

func TestPincodeBlock_UpdateBlockData(t *testing.T) {
	prompt := gofakeit.Question()
	pincode := strconv.Itoa(gofakeit.Number(1, 999999))
	points := strconv.Itoa(gofakeit.Number(1, 1000))
	block := PincodeBlock{}
	data := map[string][]string{
		"prompt":  {prompt},
		"pincode": {pincode},
		"points":  {points},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, prompt, block.Prompt)
	assert.Equal(t, pincode, block.Pincode)
	assert.Equal(t, points, strconv.Itoa(block.GetPoints()))
}

func TestPincodeBlock_ValidatePlayerInput(t *testing.T) {
	prompt := gofakeit.Question()
	pincode := "12345" // Use fixed pincode for predictable testing
	points := strconv.Itoa(gofakeit.Number(1, 1000))
	block := PincodeBlock{}
	data := map[string][]string{
		"prompt":  {prompt},
		"pincode": {pincode},
		"points":  {points},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)

	// Keep track of attempts - tests that fail validation don't increment attempts
	// Only successful validation (correct or incorrect but valid format) increments attempts

	// Test: Incorrect pincode (wrong digits)
	// Each digit provided as separate input
	// Expected behaviour: No error and no points awarded
	input := map[string][]string{
		"pincode": {"9", "8", "7", "6", "5"},
	}
	state1 := &mockPlayerState{}
	newState, err := block.ValidatePlayerInput(state1, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test: Invalid input (non-digit character)
	// Expected behaviour: No error and no points awarded (still valid format)
	input = map[string][]string{
		"pincode": {"a", "b", "c", "d", "e"},
	}
	state2 := &mockPlayerState{}
	newState, err = block.ValidatePlayerInput(state2, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test: Insufficient digits
	// Expected behaviour: Error due to length mismatch
	input = map[string][]string{
		"pincode": {"1", "2", "3"},
	}
	state3 := &mockPlayerState{}
	_, err = block.ValidatePlayerInput(state3, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pincode length does not match")

	// Test: Multiple characters in single input (invalid)
	// Expected behaviour: Error due to multi-character input
	input = map[string][]string{
		"pincode": {"12", "3", "4", "5", "6"},
	}
	state4 := &mockPlayerState{}
	_, err = block.ValidatePlayerInput(state4, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pincode must be a single character per input")

	// Test: Correct pincode (individual digits)
	// Expected behaviour: No error and points awarded
	input = map[string][]string{
		"pincode": {"1", "2", "3", "4", "5"},
	}
	state5 := &mockPlayerState{}
	newState, err = block.ValidatePlayerInput(state5, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, points, strconv.Itoa(newState.GetPointsAwarded()))

	// Check the successful attempt's data
	var newPlayerData pincodeBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &newPlayerData)
	require.NoError(t, err)
	assert.Equal(t, 1, newPlayerData.Attempts)
	assert.Len(t, newPlayerData.Guesses, 1)
	assert.Equal(t, "1", newPlayerData.Guesses[0]) // First digit saved as guess
}
