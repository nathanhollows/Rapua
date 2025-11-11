package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrArray_JSONEncoding(t *testing.T) {
	arr := models.StrArray{"Park Entrance", "Old Tower", "River Bank"}
	jsonVal, err := arr.Value()
	require.NoError(t, err)

	// Expected JSON format
	expected := `["Park Entrance","Old Tower","River Bank"]`
	assert.JSONEq(t, expected, jsonVal.(string))
}

func TestStrArray_JSONDecoding(t *testing.T) {
	var arr models.StrArray

	// Normal JSON case
	err := arr.Scan(`["Park Entrance","Old Tower","River Bank"]`)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{"Park Entrance", "Old Tower", "River Bank"}, arr)

	// Handles empty array
	err = arr.Scan(`[]`)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{}, arr)

	// Handles nil case
	err = arr.Scan(nil)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{}, arr)

	// Handles special characters
	err = arr.Scan(`["This \"quote\" test","Line\nBreak","Tab\tTest"]`)
	require.NoError(t, err)
	assert.Equal(t, models.StrArray{`This "quote" test`, "Line\nBreak", "Tab\tTest"}, arr)

	// Invalid JSON case
	err = arr.Scan(`{bad json}`)
	require.Error(t, err)
}
