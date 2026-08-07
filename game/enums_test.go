package game_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RouteStrategy ---

func TestRouteStrategy_String(t *testing.T) {
	cases := []struct {
		input game.RouteStrategy
		want  string
	}{
		{game.RouteStrategyRandomised, "Randomised Route"},
		{game.RouteStrategyFreeRoam, "Open Exploration"},
		{game.RouteStrategyOrdered, "Guided Path"},
		{game.RouteStrategySecret, "Secret"},
		{game.RouteStrategy("unknown"), "unknown"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.input.String(), "RouteStrategy(%q).String()", c.input)
	}
}

func TestRouteStrategy_Description(t *testing.T) {
	cases := []struct {
		input game.RouteStrategy
		empty bool
	}{
		{game.RouteStrategyRandomised, false},
		{game.RouteStrategyFreeRoam, false},
		{game.RouteStrategyOrdered, false},
		{game.RouteStrategySecret, false},
		{game.RouteStrategy("unknown"), true},
	}
	for _, c := range cases {
		desc := c.input.Description()
		if c.empty {
			assert.Empty(t, desc)
		} else {
			assert.NotEmpty(t, desc)
		}
	}
}

func TestParseRouteStrategy(t *testing.T) {
	valid := []struct {
		input string
		want  game.RouteStrategy
	}{
		{"randomised", game.RouteStrategyRandomised},
		{"free_roam", game.RouteStrategyFreeRoam},
		{"ordered", game.RouteStrategyOrdered},
		{"secret", game.RouteStrategySecret},
	}
	for _, c := range valid {
		got, err := game.ParseRouteStrategy(c.input)
		require.NoError(t, err, "input %q", c.input)
		assert.Equal(t, c.want, got)
	}

	_, err := game.ParseRouteStrategy("bogus")
	assert.Error(t, err)
}

func TestCompletionType_String(t *testing.T) {
	cases := []struct {
		input game.CompletionType
		want  string
	}{
		{game.CompletionAll, "All Locations"},
		{game.CompletionMinimum, "Minimum Required"},
		{game.CompletionType("unknown"), "unknown"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.input.String(), "CompletionType(%q).String()", c.input)
	}
}

func TestCompletionType_Description(t *testing.T) {
	cases := []struct {
		input game.CompletionType
		empty bool
	}{
		{game.CompletionAll, false},
		{game.CompletionMinimum, false},
		{game.CompletionType("unknown"), true},
	}
	for _, c := range cases {
		desc := c.input.Description()
		if c.empty {
			assert.Empty(t, desc)
		} else {
			assert.NotEmpty(t, desc)
		}
	}
}
