package blocks_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPincodeBlock_Getters(t *testing.T) {
	prompt := gofakeit.Question()
	pincode := strconv.Itoa(gofakeit.Number(1, 999999))
	block := blocks.PincodeBlock{
		BaseBlock: blocks.BaseBlock{
			ID:      "test-id",
			OwnerID: "location-123",
			Order:   1,
			Points:  5,
		},
		Prompt:  prompt,
		Pincode: pincode,
	}

	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetOwnerID())
	assert.Equal(t, 1, block.GetOrder())
	assert.Equal(t, 5, block.GetPoints())
}

func TestPincodeBlock_ParseData(t *testing.T) {
	prompt := gofakeit.Question()
	pincode := strconv.Itoa(gofakeit.Number(1, 999999))
	data := `{"prompt":"` + prompt + `", "pincode":"` + pincode + `"}`
	block := blocks.PincodeBlock{
		BaseBlock: blocks.BaseBlock{
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
	block := blocks.PincodeBlock{}
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
	block := blocks.PincodeBlock{}
	data := map[string][]string{
		"prompt":  {prompt},
		"pincode": {pincode},
		"points":  {points},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)

	// Test: Incorrect pincode (wrong digits, single OTP input)
	// Expected behaviour: No error and no points awarded
	input := map[string][]string{
		"pincode": {"98765"},
	}
	state1 := &blocks.MockPlayerState{}
	newState, err := block.ValidatePlayerInput(state1, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test: Incorrect pincode (non-digit characters, single OTP input)
	// Expected behaviour: No error and no points awarded
	input = map[string][]string{
		"pincode": {"abcde"},
	}
	state2 := &blocks.MockPlayerState{}
	newState, err = block.ValidatePlayerInput(state2, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test: Insufficient digits (single OTP input)
	// Expected behaviour: Error due to length mismatch
	input = map[string][]string{
		"pincode": {"123"},
	}
	state3 := &blocks.MockPlayerState{}
	_, err = block.ValidatePlayerInput(state3, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pincode length does not match")

	// Test: Correct pincode (single OTP input)
	// Expected behaviour: No error and points awarded
	input = map[string][]string{
		"pincode": {"12345"},
	}
	state4 := &blocks.MockPlayerState{}
	newState, err = block.ValidatePlayerInput(state4, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, points, strconv.Itoa(newState.GetPointsAwarded()))

	// Check the successful attempt's data
	var newPlayerData blocks.PincodeBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &newPlayerData)
	require.NoError(t, err)
	assert.Equal(t, 1, newPlayerData.Attempts)
	assert.Len(t, newPlayerData.Guesses, 1)
	assert.Equal(t, "12345", newPlayerData.Guesses[0]) // Full pincode saved as guess
}
