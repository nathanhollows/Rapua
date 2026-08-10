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
