package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpload_GetSizes(t *testing.T) {
	tests := []struct {
		name      string
		input     []models.ImageSize
		expectErr bool
		output    []models.ImageSize
	}{
		{
			name:      "Valid size",
			input:     []models.ImageSize{{Breakpoint: 480, URL: "https://cdn.example.com/image"}},
			expectErr: false,
			output:    []models.ImageSize{{Breakpoint: 480, URL: "https://cdn.example.com/image"}},
		},
		{
			name: "Valid sizes",
			input: []models.ImageSize{
				{Breakpoint: 480, URL: "https://cdn.example.com/image-480.jpg"},
				{Breakpoint: 1024, URL: "https://cdn.example.com/image-1024.jpg"},
			},
			expectErr: false,
			output: []models.ImageSize{
				{Breakpoint: 480, URL: "https://cdn.example.com/image-480.jpg"},
				{Breakpoint: 1024, URL: "https://cdn.example.com/image-1024.jpg"},
			},
		},
		{
			name:      "Empty size",
			input:     []models.ImageSize{},
			expectErr: false,
			output:    []models.ImageSize{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upload := models.Upload{}
			for _, size := range tt.input {
				err := upload.AddSize(size.Breakpoint, size.URL)
				require.NoError(t, err)
			}

			result, err := upload.GetSizes()
			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.output, result)
			}
		})
	}
}

func TestUpload_AddSize(t *testing.T) {
	tests := []struct {
		name       string
		initial    []models.ImageSize
		breakpoint int
		url        string
		expectErr  bool
		final      []models.ImageSize
	}{
		{
			name:       "Add to empty list",
			initial:    []models.ImageSize{},
			breakpoint: 1024,
			url:        "https://cdn.example.com/image-1024.jpg",
			expectErr:  false,
			final:      []models.ImageSize{{Breakpoint: 1024, URL: "https://cdn.example.com/image-1024.jpg"}},
		},
		{
			name:       "Add to existing list",
			initial:    []models.ImageSize{{Breakpoint: 480, URL: "https://cdn.example.com/image-480.jpg"}},
			breakpoint: 1440,
			url:        "https://cdn.example.com/image-1440.jpg",
			expectErr:  false,
			final: []models.ImageSize{
				{Breakpoint: 480, URL: "https://cdn.example.com/image-480.jpg"},
				{Breakpoint: 1440, URL: "https://cdn.example.com/image-1440.jpg"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upload := models.Upload{}
			for _, size := range tt.initial {
				err := upload.AddSize(size.Breakpoint, size.URL)
				require.NoError(t, err)
			}
			err := upload.AddSize(tt.breakpoint, tt.url)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				result, _ := upload.GetSizes()
				assert.Equal(t, tt.final, result)
			}
		})
	}
}
