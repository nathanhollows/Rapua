package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/stretchr/testify/assert"
)

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

func TestLocation_HasCluesContext(t *testing.T) {
	tests := []struct {
		name     string
		location models.Location
		want     bool
	}{
		{
			name: "Location with clues block",
			location: models.Location{
				Blocks: []models.Block{
					{Context: blocks.ContextLocationClues},
				},
			},
			want: true,
		},
		{
			name: "Location with multiple blocks including clues",
			location: models.Location{
				Blocks: []models.Block{
					{Context: blocks.ContextLocationContent},
					{Context: blocks.ContextLocationClues},
					{Context: blocks.ContextCheckpoint},
				},
			},
			want: true,
		},
		{
			name: "Location with only content blocks",
			location: models.Location{
				Blocks: []models.Block{
					{Context: blocks.ContextLocationContent},
					{Context: blocks.ContextCheckpoint},
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
			got := tt.location.HasCluesContext()
			assert.Equal(t, tt.want, got)
		})
	}
}
