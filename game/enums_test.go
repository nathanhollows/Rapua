package game_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v7/game"
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

// --- NavigationMode ---

func TestNavigationMode_String(t *testing.T) {
	cases := []struct {
		input game.NavigationMode
		want  string
	}{
		{game.NavigationMap, "Map Only"},
		{game.NavigationLabelledMap, "Labelled Map"},
		{game.NavigationList, "Location List"},
		{game.NavigationCustom, "Custom Clues"},
		{game.NavigationTasks, "Tasks"},
		{game.NavigationMode("unknown"), "unknown"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.input.String(), "NavigationMode(%q).String()", c.input)
	}
}

func TestNavigationMode_Description(t *testing.T) {
	cases := []struct {
		input game.NavigationMode
		empty bool
	}{
		{game.NavigationMap, false},
		{game.NavigationLabelledMap, false},
		{game.NavigationList, false},
		{game.NavigationCustom, false},
		{game.NavigationTasks, false},
		{game.NavigationMode("unknown"), true},
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

func TestParseNavigationMode(t *testing.T) {
	valid := []struct {
		input string
		want  game.NavigationMode
	}{
		{"map", game.NavigationMap},
		{"labelled_map", game.NavigationLabelledMap},
		{"location_list", game.NavigationList},
		{"custom", game.NavigationCustom},
		{"tasks", game.NavigationTasks},
	}
	for _, c := range valid {
		got, err := game.ParseNavigationMode(c.input)
		require.NoError(t, err, "input %q", c.input)
		assert.Equal(t, c.want, got)
	}

	_, err := game.ParseNavigationMode("bogus")
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
