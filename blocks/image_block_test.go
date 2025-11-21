package blocks_test

import (
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageBlock_Getters(t *testing.T) {
	block := blocks.ImageBlock{
		BaseBlock: blocks.BaseBlock{
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
	url := gofakeit.URL()
	caption := gofakeit.Sentence(5)
	link := gofakeit.URL()

	block := blocks.ImageBlock{}
	data := map[string][]string{
		"url":     {url},
		"caption": {caption},
		"link":    {link},
	}
	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, url, block.URL)
	assert.Equal(t, caption, block.Caption)
	assert.Equal(t, link, block.Link)

	// Invalid URL
	err = block.UpdateBlockData(map[string][]string{"URL": {"invalid"}})
	require.Error(t, err)
}
