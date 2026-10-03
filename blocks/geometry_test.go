package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func closedRing() blocks.LinearRing {
	return blocks.LinearRing{
		blocks.NewPosition(-45.87, 170.50),
		blocks.NewPosition(-45.88, 170.51),
		blocks.NewPosition(-45.89, 170.49),
		blocks.NewPosition(-45.87, 170.50),
	}
}

// Getting the longitude-first convention backwards puts every point on the
// wrong side of the planet.
func TestPosition_Order(t *testing.T) {
	p := blocks.NewPosition(-45.8788, 170.5028)
	assert.InDelta(t, -45.8788, p.Lat(), 0.00001)
	assert.InDelta(t, 170.5028, p.Lng(), 0.00001)
	assert.InDelta(t, 170.5028, p[0], 0.00001, "longitude is stored first")
	assert.InDelta(t, -45.8788, p[1], 0.00001, "latitude is stored second")
}

// A short array would zero-fill into the fixed-size Position, putting the
// latitude on the equator without complaint.
func TestPosition_RejectsShortCoordinates(t *testing.T) {
	var p blocks.Position
	require.ErrorIs(t, json.Unmarshal([]byte(`[170.5028]`), &p), blocks.ErrPositionTooShort)
	require.ErrorIs(t, json.Unmarshal([]byte(`[]`), &p), blocks.ErrPositionTooShort)
}

// RFC 7946 allows a third element for altitude, which this does not use.
func TestPosition_DiscardsAltitude(t *testing.T) {
	var p blocks.Position
	require.NoError(t, json.Unmarshal([]byte(`[170.5028,-45.8788,12.5]`), &p))
	assert.InDelta(t, 170.5028, p.Lng(), 0.00001)
	assert.InDelta(t, -45.8788, p.Lat(), 0.00001)
}

// Pins the authored wire format: a circle is a centre and a radius, not GeoJSON.
func TestGeometry_CircleWireFormat(t *testing.T) {
	data, err := json.Marshal(blocks.NewCircle(-41.2865, 174.7762, 100))
	require.NoError(t, err)

	assert.JSONEq(t, `{"type":"circle","center":[174.7762,-41.2865],"radius":100}`, string(data))
}

func TestGeometry_CircleRoundTrip(t *testing.T) {
	var g blocks.Geometry
	require.NoError(t, json.Unmarshal(
		[]byte(`{"type":"circle","center":[174.7762,-41.2865],"radius":100}`), &g))

	assert.Equal(t, blocks.GeometryCircle, g.Type)
	assert.InDelta(t, -41.2865, g.Center.Lat(), 0.00001)
	assert.InDelta(t, 174.7762, g.Center.Lng(), 0.00001)
	assert.InDelta(t, 100, g.Radius, 0.001)
	require.NoError(t, g.Validate())
}

// A boundary stays GeoJSON so a map draw control round-trips it untouched. This
// is the literal shape Mapbox Draw hands over.
func TestGeometry_PolygonIsGeoJSON(t *testing.T) {
	raw := `{"type":"Polygon","coordinates":[[
		[170.5028,-45.8788],[170.5040,-45.8790],[170.5030,-45.8800],[170.5028,-45.8788]
	]]}`

	var g blocks.Geometry
	require.NoError(t, json.Unmarshal([]byte(raw), &g))

	assert.Equal(t, blocks.GeometryPolygon, g.Type)
	require.Len(t, g.LinearRings, 1)
	require.Len(t, g.LinearRings[0], 4)
	assert.InDelta(t, -45.8788, g.LinearRings[0][0].Lat(), 0.00001)
	require.NoError(t, g.Validate())

	out, err := json.Marshal(&g)
	require.NoError(t, err)
	assert.JSONEq(t, raw, string(out), "a polygon survives the round trip unchanged")
}

func TestGeometry_ZeroMarshalsAsNull(t *testing.T) {
	data, err := json.Marshal(&blocks.Geometry{})
	require.NoError(t, err)
	assert.JSONEq(t, "null", string(data))
}

func TestGeometry_UnmarshalNull(t *testing.T) {
	g := *blocks.NewCircle(-41.28, 174.77, 100)
	require.NoError(t, json.Unmarshal([]byte("null"), &g))
	assert.Equal(t, blocks.GeometryCircle, g.Type, "null leaves the value untouched")
}

func TestGeometry_UnmarshalUnknownType(t *testing.T) {
	var g blocks.Geometry
	err := json.Unmarshal([]byte(`{"type":"LineString","coordinates":[[1,2],[3,4]]}`), &g)
	require.ErrorIs(t, err, blocks.ErrGeometryType)
}

// IsZero reports true for a type it does not recognise, so neither write path
// may use it as the only gate. A broken shape would be stored as NULL and
// reported as success. The read path already errors; the write path is where the
// loss would happen.
func TestGeometry_UnknownTypeIsNotWrittenAsNull(t *testing.T) {
	g := blocks.Geometry{
		Type:   blocks.GeometryType("LineString"),
		Center: blocks.NewPosition(-41.2865, 174.7762),
		Radius: 20,
	}

	_, err := json.Marshal(&g)
	require.ErrorIs(t, err, blocks.ErrGeometryType)

	_, err = g.Value()
	require.ErrorIs(t, err, blocks.ErrGeometryType)
}

func TestGeometry_IsZero(t *testing.T) {
	var nilGeom *blocks.Geometry
	assert.True(t, nilGeom.IsZero(), "nil geometry")
	assert.True(t, (&blocks.Geometry{}).IsZero(), "empty geometry")
	assert.True(t, (&blocks.Geometry{Type: blocks.GeometryPolygon}).IsZero(), "polygon with no rings")
	assert.False(t, blocks.NewCircle(-45.87, 170.5, 20).IsZero())
	assert.False(t, blocks.NewPolygon(closedRing()).IsZero())
}

func TestGeometry_Validate(t *testing.T) {
	cases := []struct {
		name    string
		geom    *blocks.Geometry
		wantErr error
	}{
		{name: "nil", geom: nil},
		{name: "empty", geom: &blocks.Geometry{}},
		{name: "valid circle", geom: blocks.NewCircle(-45.8788, 170.5028, 20)},
		{name: "valid polygon", geom: blocks.NewPolygon(closedRing())},
		{
			name:    "circle without a radius",
			geom:    blocks.NewCircle(-45.87, 170.5, 0),
			wantErr: blocks.ErrCircleNeedsRadius,
		},
		{
			name:    "circle with a negative radius",
			geom:    blocks.NewCircle(-45.87, 170.5, -5),
			wantErr: blocks.ErrCircleNeedsRadius,
		},
		{
			name:    "latitude out of range",
			geom:    blocks.NewCircle(91, 170.5, 20),
			wantErr: blocks.ErrGeometryLatitude,
		},
		{
			name:    "longitude out of range",
			geom:    blocks.NewCircle(-45.87, 181, 20),
			wantErr: blocks.ErrGeometryLongitude,
		},
		{
			name:    "polygon with no rings",
			geom:    &blocks.Geometry{Type: blocks.GeometryPolygon, LinearRings: []blocks.LinearRing{{}}},
			wantErr: blocks.ErrLinearRingTooShort,
		},
		{
			name: "ring too short",
			geom: blocks.NewPolygon(blocks.LinearRing{
				blocks.NewPosition(-45.87, 170.50),
				blocks.NewPosition(-45.88, 170.51),
				blocks.NewPosition(-45.87, 170.50),
			}),
			wantErr: blocks.ErrLinearRingTooShort,
		},
		{
			name: "ring not closed",
			geom: blocks.NewPolygon(blocks.LinearRing{
				blocks.NewPosition(-45.87, 170.50),
				blocks.NewPosition(-45.88, 170.51),
				blocks.NewPosition(-45.89, 170.49),
				blocks.NewPosition(-45.86, 170.52),
			}),
			wantErr: blocks.ErrLinearRingNotClosed,
		},
		{
			name: "position in ring out of range",
			geom: blocks.NewPolygon(blocks.LinearRing{
				blocks.NewPosition(-45.87, 170.50),
				blocks.NewPosition(-95, 170.51),
				blocks.NewPosition(-45.89, 170.49),
				blocks.NewPosition(-45.87, 170.50),
			}),
			wantErr: blocks.ErrGeometryLatitude,
		},
		{
			name:    "unknown type",
			geom:    &blocks.Geometry{Type: blocks.GeometryType("LineString"), Radius: 1},
			wantErr: blocks.ErrGeometryType,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.geom.Validate()
			if c.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, c.wantErr)
		})
	}
}

func TestGeometry_ValueNilWhenEmpty(t *testing.T) {
	v, err := (&blocks.Geometry{}).Value()
	require.NoError(t, err)
	assert.Nil(t, v, "empty geometry stores as SQL NULL")
}

func TestGeometry_ValueScanRoundTrip(t *testing.T) {
	for _, original := range []*blocks.Geometry{
		blocks.NewCircle(-41.2865, 174.7762, 100),
		blocks.NewPolygon(closedRing()),
	} {
		v, err := original.Value()
		require.NoError(t, err)
		require.IsType(t, "", v)

		var restored blocks.Geometry
		require.NoError(t, restored.Scan(v))
		assert.Equal(t, *original, restored)

		// Scanning from []byte is the other shape drivers hand back.
		var fromBytes blocks.Geometry
		require.NoError(t, fromBytes.Scan([]byte(v.(string))))
		assert.Equal(t, *original, fromBytes)
	}
}

func TestGeometry_ScanEdgeCases(t *testing.T) {
	var g blocks.Geometry
	require.NoError(t, g.Scan(nil), "nil leaves geometry untouched")
	assert.True(t, g.IsZero())

	require.NoError(t, g.Scan(""), "empty string leaves geometry untouched")
	assert.True(t, g.IsZero())

	assert.Error(t, g.Scan(42), "unsupported type errors")
}

func TestGeometry_CircleContains(t *testing.T) {
	within100m := blocks.NewCircle(octagon.Lat(), octagon.Lng(), 100)

	assert.True(t, within100m.Contains(octagon), "the centre is inside")
	assert.False(t, within100m.Contains(railway), "530m away is outside a 100m circle")
	assert.True(t,
		blocks.NewCircle(octagon.Lat(), octagon.Lng(), 1000).Contains(railway),
		"530m away is inside a 1km circle")
}

func TestGeometry_ContainsNothingWhenEmpty(t *testing.T) {
	var nilGeom *blocks.Geometry
	assert.False(t, nilGeom.Contains(octagon))
	assert.False(t, (&blocks.Geometry{}).Contains(octagon))
	assert.False(t, (&blocks.Geometry{Type: blocks.GeometryPolygon}).Contains(octagon))
}

func TestGeometry_MetresOutsideCircle(t *testing.T) {
	within100m := blocks.NewCircle(octagon.Lat(), octagon.Lng(), 100)

	assert.Zero(t, within100m.MetresOutside(octagon), "inside is not outside")
	// The railway station is ~530m away, so ~430m beyond a 100m circle.
	assert.InDelta(t, 430, within100m.MetresOutside(railway), 60)
}

func TestGeometry_MetresOutsideEmpty(t *testing.T) {
	var nilGeom *blocks.Geometry
	assert.Zero(t, nilGeom.MetresOutside(octagon))
	assert.Zero(t, (&blocks.Geometry{}).MetresOutside(octagon))
}

// A position in a courtyard is outside the building, so it is told how far it is
// back to the wall rather than nothing.
func TestGeometry_MetresOutsideHole(t *testing.T) {
	withCourtyard := &blocks.Geometry{
		Type: blocks.GeometryPolygon,
		LinearRings: []blocks.LinearRing{
			{
				blocks.NewPosition(-45.880, 170.500),
				blocks.NewPosition(-45.880, 170.510),
				blocks.NewPosition(-45.870, 170.510),
				blocks.NewPosition(-45.870, 170.500),
				blocks.NewPosition(-45.880, 170.500),
			},
			{
				blocks.NewPosition(-45.876, 170.504),
				blocks.NewPosition(-45.876, 170.506),
				blocks.NewPosition(-45.874, 170.506),
				blocks.NewPosition(-45.874, 170.504),
				blocks.NewPosition(-45.876, 170.504),
			},
		},
	}

	inCourtyard := blocks.NewPosition(-45.875, 170.505)
	assert.False(t, withCourtyard.Contains(inCourtyard))
	assert.Greater(t, withCourtyard.MetresOutside(inCourtyard), 0.0, "the wall is a measurable distance away")
	assert.Less(t, withCourtyard.MetresOutside(inCourtyard), 200.0)
}

// A polygon measures to its nearest edge, so a shape reports a near miss the same
// way a circle does rather than staying silent.
func TestGeometry_MetresOutsidePolygon(t *testing.T) {
	// A box spanning 0.004 degrees of latitude, about 445m tall.
	box := blocks.NewPolygon(blocks.LinearRing{
		blocks.NewPosition(-45.876, 170.501),
		blocks.NewPosition(-45.876, 170.506),
		blocks.NewPosition(-45.872, 170.506),
		blocks.NewPosition(-45.872, 170.501),
		blocks.NewPosition(-45.876, 170.501),
	})

	assert.Zero(t, box.MetresOutside(octagon), "inside reports nothing")

	// 0.001 degrees of latitude north of the top edge is about 111m.
	justNorth := blocks.NewPosition(-45.871, 170.5035)
	assert.InDelta(t, 111, box.MetresOutside(justNorth), 15)

	// Far away should read far away, not zero.
	assert.Greater(t, box.MetresOutside(portobello), 10000.0)
}

func TestGeometry_PolygonContains(t *testing.T) {
	// A square roughly around the Octagon.
	square := blocks.NewPolygon(blocks.LinearRing{
		blocks.NewPosition(-45.876, 170.501),
		blocks.NewPosition(-45.876, 170.506),
		blocks.NewPosition(-45.872, 170.506),
		blocks.NewPosition(-45.872, 170.501),
		blocks.NewPosition(-45.876, 170.501),
	})

	assert.True(t, square.Contains(octagon))
	assert.False(t, square.Contains(railway))
	assert.False(t, square.Contains(portobello))
}

// GeoJSON treats a second ring as a hole, so the courtyard in the middle of a
// building is outside the building.
func TestGeometry_PolygonHoleIsOutside(t *testing.T) {
	withCourtyard := &blocks.Geometry{
		Type: blocks.GeometryPolygon,
		LinearRings: []blocks.LinearRing{
			{
				blocks.NewPosition(-45.880, 170.500),
				blocks.NewPosition(-45.880, 170.510),
				blocks.NewPosition(-45.870, 170.510),
				blocks.NewPosition(-45.870, 170.500),
				blocks.NewPosition(-45.880, 170.500),
			},
			{
				blocks.NewPosition(-45.876, 170.504),
				blocks.NewPosition(-45.876, 170.506),
				blocks.NewPosition(-45.874, 170.506),
				blocks.NewPosition(-45.874, 170.504),
				blocks.NewPosition(-45.876, 170.504),
			},
		},
	}

	assert.True(t, withCourtyard.Contains(blocks.NewPosition(-45.8790, 170.5020)), "inside the building")
	assert.False(t, withCourtyard.Contains(blocks.NewPosition(-45.8750, 170.5050)), "inside the courtyard")
}

// A circle with no radius is a point, and nothing can stand on a point.
func TestGeometry_ZeroRadiusContainsNothing(t *testing.T) {
	assert.False(t, blocks.NewCircle(octagon.Lat(), octagon.Lng(), 0).Contains(octagon))
}

// Dunedin, so the distances below are real ones on the ground.
var (
	octagon    = blocks.NewPosition(-45.8742, 170.5036)
	railway    = blocks.NewPosition(-45.8748, 170.5104) // ~530m east of the Octagon
	portobello = blocks.NewPosition(-45.8500, 170.6600) // ~12km away
)

func TestMetresBetween(t *testing.T) {
	assert.InDelta(t, 0, blocks.MetresBetween(octagon, octagon), 0.001)
	assert.InDelta(t, 530, blocks.MetresBetween(octagon, railway), 40)
	assert.InDelta(t, 12600, blocks.MetresBetween(octagon, portobello), 400)

	// Distance does not care which way round it is asked.
	assert.InDelta(t,
		blocks.MetresBetween(octagon, railway),
		blocks.MetresBetween(railway, octagon), 0.001)
}
