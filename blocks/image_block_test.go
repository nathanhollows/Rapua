package blocks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageBlock_Getters(t *testing.T) {
	block := ImageBlock{
		BaseBlock: BaseBlock{
			ID:         "test-id",
			LocationID: "location-123",
			Order:      0,
			Points:     0,
		},
		URL: "https://placecage.lucidinternets.com/500/400",
	}

	assert.Equal(t, "Image", block.GetName())
	assert.Equal(t, "image", block.GetType())
	assert.Equal(t, "test-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, 0, block.GetOrder())
	assert.Equal(t, 0, block.GetPoints())
}

func TestImageBlock_UpdateBlockData(t *testing.T) {
	block := ImageBlock{}
	data := map[string][]string{
		"url":     {"https://placecage.lucidinternets.com/500/400"},
		"caption": {"A test caption"},
		"link":    {"https://placecage.lucidinternets.com/500/400"},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "https://placecage.lucidinternets.com/500/400", block.URL)
	assert.Equal(t, "A test caption", block.Caption)
	assert.Equal(t, "https://placecage.lucidinternets.com/500/400", block.Link)

	// Invalid URL
	err = block.UpdateBlockData(map[string][]string{"URL": {"invalid"}})
	require.Error(t, err)
}
