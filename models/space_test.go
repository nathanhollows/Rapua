package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// A space that moves cannot be proved by GPS: the coordinates recorded when it
// was registered say nothing about where it is now. The "moves" badge and the
// advertised proof methods have to agree.
func TestSpace_ProofMethodsDropGPSWhenMobile(t *testing.T) {
	cases := []struct {
		name   string
		kind   game.SpaceKind
		mobile bool
		want   []game.ProofMethod
	}{
		{
			name: "fixed point",
			kind: game.SpaceKindPoint,
			want: []game.ProofMethod{game.ProofMethodGPS, game.ProofMethodQR, game.ProofMethodNFC},
		},
		{
			name:   "point that moves",
			kind:   game.SpaceKindPoint,
			mobile: true,
			want:   []game.ProofMethod{game.ProofMethodQR, game.ProofMethodNFC},
		},
		{
			name: "fixed area",
			kind: game.SpaceKindArea,
			want: []game.ProofMethod{game.ProofMethodGPS, game.ProofMethodQR},
		},
		{
			name:   "area that moves",
			kind:   game.SpaceKindArea,
			mobile: true,
			want:   []game.ProofMethod{game.ProofMethodQR},
		},
		{
			name: "object never had GPS",
			kind: game.SpaceKindObject,
			want: []game.ProofMethod{game.ProofMethodQR, game.ProofMethodNFC},
		},
		{
			name:   "object that moves",
			kind:   game.SpaceKindObject,
			mobile: true,
			want:   []game.ProofMethod{game.ProofMethodQR, game.ProofMethodNFC},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			space := models.Space{Kind: c.kind, Mobile: c.mobile}
			assert.Equal(t, c.want, space.ProofMethods())
		})
	}
}

func TestSpace_AllowsMethod(t *testing.T) {
	fixed := models.Space{Kind: game.SpaceKindPoint}
	assert.True(t, fixed.AllowsMethod(game.ProofMethodGPS))
	assert.True(t, fixed.AllowsMethod(game.ProofMethodQR))

	moving := models.Space{Kind: game.SpaceKindPoint, Mobile: true}
	assert.False(t, moving.AllowsMethod(game.ProofMethodGPS), "a space that moves cannot be found by GPS")
	assert.True(t, moving.AllowsMethod(game.ProofMethodQR))

	assert.False(t, fixed.AllowsMethod(game.ProofMethod("bluetooth")))
}

func TestSpace_HasCoordinates(t *testing.T) {
	assert.False(t, (&models.Space{}).HasCoordinates(), "no geometry means no position")
	assert.False(t, (&models.Space{
		Kind:     game.SpaceKindObject,
		Geometry: nil,
	}).HasCoordinates())
	assert.True(t, (&models.Space{
		Kind:     game.SpaceKindPoint,
		Geometry: game.NewPointGeometry(-45.8788, 170.5028, 20),
	}).HasCoordinates())
}
