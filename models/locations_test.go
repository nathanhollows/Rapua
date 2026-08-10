package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Location", "my-location"},
		{"UPPERCASE", "uppercase"},
		{"hello world", "hello-world"},
		{"multiple   spaces", "multiple-spaces"},
		{"Store #1", "store-1"},
		{"50% Off!", "50-off"},
		{"Hello!!!World", "hello-world"},
		{"a-b-c", "a-b-c"},
		{"-leading-and-trailing-", "leading-and-trailing"},
		{"---", ""},
		{"", ""},
		{"   ", ""},
		{"Citrus Collection", "citrus-collection"},
		{"Ōtākou Whakaihu Waka", "otakou-whakaihu-waka"},
		{"Tāwhaki Pou Whenua", "tawhaki-pou-whenua"},
		{"Te Rangihīroa College", "te-rangihiroa-college"},
		{"Café Crème", "cafe-creme"},
		{"Peñíscola", "peniscola"},
		{"Ångström", "angstrom"},
		{"Old Government Buildings", "old-government-buildings"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := models.Slugify(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocation_HasCoordinates(t *testing.T) {
	tests := []struct {
		name     string
		location models.Location
		want     bool
	}{
		{
			name: "Location with mapped coordinates",
			location: models.Location{
				Marker: models.Marker{
					Lat: -41.2865,
					Lng: 174.7762,
				},
			},
			want: true,
		},
		{
			name: "Location without coordinates (zero values)",
			location: models.Location{
				Marker: models.Marker{
					Lat: 0,
					Lng: 0,
				},
			},
			want: false,
		},
		{
			name: "Location with partial coordinates (only latitude)",
			location: models.Location{
				Marker: models.Marker{
					Lat: -41.2865,
					Lng: 0,
				},
			},
			want: false,
		},
		{
			name: "Location with partial coordinates (only longitude)",
			location: models.Location{
				Marker: models.Marker{
					Lat: 0,
					Lng: 174.7762,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.location.HasCoordinates()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocation_HasNavigationContext(t *testing.T) {
	tests := []struct {
		name     string
		location models.Location
		want     bool
	}{
		{
			name: "Location with navigation block",
			location: models.Location{
				Blocks: []models.Block{
					{Context: blocks.ContextNavigation},
				},
			},
			want: true,
		},
		{
			name: "Location with multiple blocks including navigation",
			location: models.Location{
				Blocks: []models.Block{
					{Context: blocks.ContextLocationContent},
					{Context: blocks.ContextNavigation},
				},
			},
			want: true,
		},
		{
			name: "Location with only content blocks",
			location: models.Location{
				Blocks: []models.Block{
					{Context: blocks.ContextLocationContent},
				},
			},
			want: false,
		},
		{
			name: "Location with no blocks",
			location: models.Location{
				Blocks: []models.Block{},
			},
			want: false,
		},
		{
			name: "Location with nil blocks",
			location: models.Location{
				Blocks: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.location.HasNavigationContext()
			assert.Equal(t, tt.want, got)
		})
	}
}
