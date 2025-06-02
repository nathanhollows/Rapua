package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrArray_JSONEncoding(t *testing.T) {
	arr := StrArray{"Park Entrance", "Old Tower", "River Bank"}
	jsonVal, err := arr.Value()
	assert.NoError(t, err)

	// Expected JSON format
	expected := `["Park Entrance","Old Tower","River Bank"]`
	assert.JSONEq(t, expected, jsonVal.(string))
}

func TestStrArray_JSONDecoding(t *testing.T) {
	var arr StrArray

	// Normal JSON case
	err := arr.Scan(`["Park Entrance","Old Tower","River Bank"]`)
	assert.NoError(t, err)
	assert.Equal(t, StrArray{"Park Entrance", "Old Tower", "River Bank"}, arr)

	// Handles empty array
	err = arr.Scan(`[]`)
	assert.NoError(t, err)
	assert.Equal(t, StrArray{}, arr)

	// Handles nil case
	err = arr.Scan(nil)
	assert.NoError(t, err)
	assert.Equal(t, StrArray{}, arr)

	// Handles special characters
	err = arr.Scan(`["This \"quote\" test","Line\nBreak","Tab\tTest"]`)
	assert.NoError(t, err)
	assert.Equal(t, StrArray{`This "quote" test`, "Line\nBreak", "Tab\tTest"}, arr)

	// Invalid JSON case
	err = arr.Scan(`{bad json}`)
	assert.Error(t, err)
}
