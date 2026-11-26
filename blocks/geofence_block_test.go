package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func geofenceAt() *blocks.GeofenceBlock {
	return &blocks.GeofenceBlock{
		Area: blocks.NewCircle(octagon.Lat(), octagon.Lng(), 100),
	}
}

func fix(p blocks.Position, accuracy string) map[string][]string {
	return map[string][]string{
		"lat":      {formatCoord(p.Lat())},
		"lng":      {formatCoord(p.Lng())},
		"accuracy": {accuracy},
	}
}

func formatCoord(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestGeofenceBlock_InsideCompletes(t *testing.T) {
	b := geofenceAt()
	state, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "10"))
	require.NoError(t, err)
	assert.True(t, state.IsComplete())
}

func TestGeofenceBlock_OutsideDoesNotComplete(t *testing.T) {
	b := geofenceAt()
	state, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(railway, "10"))
	require.NoError(t, err, "being elsewhere is not an error, just not done")
	assert.False(t, state.IsComplete())

	var data struct {
		Attempts   int     `json:"attempts"`
		MetresAway float64 `json:"metres_away"`
	}
	require.NoError(t, json.Unmarshal(state.GetPlayerData(), &data))
	assert.Equal(t, 1, data.Attempts)
	assert.InDelta(t, 430, data.MetresAway, 60, "how far outside separates a near miss from the wrong place")
}

// A vague reading is refused rather than guessed at: a fix good to half a
// kilometre would otherwise let someone pass from home.
func TestGeofenceBlock_VagueFixIsRefused(t *testing.T) {
	b := geofenceAt()
	_, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "500"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too vague")
}

func TestGeofenceBlock_AccuracyLimitIsAuthorable(t *testing.T) {
	b := geofenceAt()
	b.MaxAccuracy = 800

	state, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "500"))
	require.NoError(t, err, "an author can accept a vaguer fix if the place suits it")
	assert.True(t, state.IsComplete())
}

// An imported block without a limit must still test positions.
func TestGeofenceBlock_UnsetAccuracyUsesTheDefault(t *testing.T) {
	b := geofenceAt()

	_, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "40"))
	require.NoError(t, err, "40m is a poor but real fix")

	_, err = b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "60"))
	require.Error(t, err, "60m is past the 50m default")
}

func TestGeofenceBlock_RejectsUnreadableInput(t *testing.T) {
	cases := map[string]map[string][]string{
		"no position":      {"accuracy": {"10"}},
		"no accuracy":      {"lat": {"-45.8742"}, "lng": {"170.5036"}},
		"lat not a number": {"lat": {"here"}, "lng": {"170.5036"}, "accuracy": {"10"}},
		"impossible lat":   {"lat": {"91"}, "lng": {"170.5036"}, "accuracy": {"10"}},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := geofenceAt().ValidatePlayerInput(&blocks.MockPlayerState{}, input)
			require.Error(t, err)
		})
	}
}

// An author who has not drawn the area yet must not ship a block anyone passes.
func TestGeofenceBlock_NoAreaNeverCompletes(t *testing.T) {
	b := &blocks.GeofenceBlock{}
	_, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "10"))
	require.Error(t, err)
}

func TestGeofenceBlock_PolygonArea(t *testing.T) {
	b := &blocks.GeofenceBlock{
		Area: blocks.NewPolygon(blocks.LinearRing{
			blocks.NewPosition(-45.876, 170.501),
			blocks.NewPosition(-45.876, 170.506),
			blocks.NewPosition(-45.872, 170.506),
			blocks.NewPosition(-45.872, 170.501),
			blocks.NewPosition(-45.876, 170.501),
		}),
	}

	state, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(octagon, "10"))
	require.NoError(t, err)
	assert.True(t, state.IsComplete())

	state, err = b.ValidatePlayerInput(&blocks.MockPlayerState{}, fix(railway, "10"))
	require.NoError(t, err)
	assert.False(t, state.IsComplete())
}

func TestGeofenceBlock_UpdateBlockData(t *testing.T) {
	var b blocks.GeofenceBlock
	require.NoError(t, b.UpdateBlockData(map[string][]string{
		"prompt":       {"Stand by the fountain"},
		"area":         {`{"type":"circle","center":[170.5036,-45.8742],"radius":40}`},
		"max_accuracy": {"25"},
		"points":       {"5"},
	}))

	assert.Equal(t, "Stand by the fountain", b.Prompt)
	require.NotNil(t, b.Area)
	assert.Equal(t, blocks.GeometryCircle, b.Area.Type)
	assert.InDelta(t, 40, b.Area.Radius, 0.001)
	assert.InDelta(t, 25, b.MaxAccuracy, 0.001)
	assert.Equal(t, 5, b.GetPoints())
}

func TestGeofenceBlock_UpdateBlockDataRejectsBadInput(t *testing.T) {
	cases := map[string]map[string][]string{
		"area is not json":     {"area": {"{nope"}},
		"area is not a shape":  {"area": {`{"type":"LineString","coordinates":[[1,2],[3,4]]}`}},
		"circle has no radius": {"area": {`{"type":"circle","center":[170.5,-45.8]}`}},
		"accuracy is zero":     {"max_accuracy": {"0"}},
		"accuracy is words":    {"max_accuracy": {"very"}},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var b blocks.GeofenceBlock
			require.Error(t, b.UpdateBlockData(input))
		})
	}
}

// The editor saves on every keystroke, so a partly-filled block must still save.
func TestGeofenceBlock_UpdateBlockDataAcceptsPromptAlone(t *testing.T) {
	var b blocks.GeofenceBlock
	require.NoError(t, b.UpdateBlockData(map[string][]string{"prompt": {"Stand by the fountain"}}))
	assert.Equal(t, "Stand by the fountain", b.Prompt)
	assert.Nil(t, b.Area)
}
