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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Equal(t, prompt, block.Prompt)
	assert.Equal(t, pincode, block.Pincode)
	assert.Equal(t, points, strconv.Itoa(block.GetPoints()))
}

func TestPincodeBlock_ValidatePlayerInput(t *testing.T) {
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
	assert.NoError(t, err)

	state := &mockPlayerState{}

	// Test: Incorrect pincode
	// Guess is valid but incorrect
	// Expected behaviour: No error and no points awarded
	input := map[string][]string{
		"pincode": {"1234"},
	}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test: Non-integer pincode
	// Guess is valid but incorrect
	// Expected behaviour: No error and no points awarded
	input = map[string][]string{
		"pincode": {"abc"},
	}
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.False(t, newState.IsComplete())
	assert.Equal(t, 0, newState.GetPointsAwarded())

	// Test: Correct pincode
	// Guess is valid and correct
	// Expected behaviour: No error and points awarded
	input = map[string][]string{
		"pincode": {pincode},
	}
	newState, err = block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
	assert.Equal(t, points, strconv.Itoa(newState.GetPointsAwarded()))

	var newPlayerData pincodeBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &newPlayerData)
	require.NoError(t, err)
	assert.Equal(t, 3, newPlayerData.Attempts)
	assert.Equal(t, 3, len(newPlayerData.Guesses))
}
