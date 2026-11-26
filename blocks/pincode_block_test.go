package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A guess arrives as one value from a single field, or one value per digit box,
// in order. Either way it is the same guess.
func TestPincodeBlock_GuessFromSeparateBoxesIsJoined(t *testing.T) {
	block := &blocks.PincodeBlock{Pincode: "4417"}

	cases := map[string][]string{
		"one field":           {"4417"},
		"one value per digit": {"4", "4", "1", "7"},
	}
	for name, guess := range cases {
		state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, map[string][]string{"pincode": guess})
		require.NoError(t, err, name)
		assert.True(t, state.IsComplete(), name)
	}

	state, err := block.ValidatePlayerInput(
		&blocks.MockPlayerState{},
		map[string][]string{"pincode": {"4", "4", "1", "8"}},
	)
	require.NoError(t, err)
	assert.False(t, state.IsComplete(), "a wrong guess is not complete")

	_, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, map[string][]string{"pincode": {"", "", "", ""}})
	require.Error(t, err, "empty boxes are not a guess")
}

// Longer codes work the same way.
func TestPincodeBlock_LongCode(t *testing.T) {
	block := &blocks.PincodeBlock{Pincode: "20260315"}

	state, err := block.ValidatePlayerInput(
		&blocks.MockPlayerState{},
		map[string][]string{"pincode": {"2", "0", "2", "6", "0", "3", "1", "5"}},
	)

	require.NoError(t, err)
	assert.True(t, state.IsComplete())
}
