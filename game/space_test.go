package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SpaceKind ---

func TestSpaceKind_Valid(t *testing.T) {
	cases := []struct {
		input game.SpaceKind
		want  bool
	}{
		{game.SpaceKindPoint, true},
		{game.SpaceKindArea, true},
		{game.SpaceKindObject, true},
		{game.SpaceKind("location"), false},
		{game.SpaceKind(""), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.input.Valid(), "SpaceKind(%q).Valid()", c.input)
	}
}

func TestSpaceKind_String(t *testing.T) {
	cases := []struct {
		input game.SpaceKind
		want  string
	}{
		{game.SpaceKindPoint, "Point"},
		{game.SpaceKindArea, "Area"},
		{game.SpaceKindObject, "Object"},
		{game.SpaceKind("unknown"), "unknown"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.input.String(), "SpaceKind(%q).String()", c.input)
	}
}

func TestSpaceKind_Description(t *testing.T) {
	for _, k := range []game.SpaceKind{game.SpaceKindPoint, game.SpaceKindArea, game.SpaceKindObject} {
		assert.NotEmpty(t, k.Description(), "SpaceKind(%q).Description()", k)
	}
	assert.Empty(t, game.SpaceKind("unknown").Description())
}

func TestParseSpaceKind(t *testing.T) {
	for _, s := range []string{"point", "area", "object"} {
		got, err := game.ParseSpaceKind(s)
		require.NoError(t, err, "ParseSpaceKind(%q)", s)
		assert.Equal(t, game.SpaceKind(s), got)
	}
	_, err := game.ParseSpaceKind("venue")
	assert.Error(t, err)
}

// --- ProofMethod ---

func TestProofMethod_Valid(t *testing.T) {
	cases := []struct {
		input game.ProofMethod
		want  bool
	}{
		{game.ProofMethodGPS, true},
		{game.ProofMethodQR, true},
		{game.ProofMethodNFC, true},
		{game.ProofMethod("bluetooth"), false},
		{game.ProofMethod(""), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.input.Valid(), "ProofMethod(%q).Valid()", c.input)
	}
}

func TestParseProofMethod(t *testing.T) {
	for _, s := range []string{"gps", "qr", "nfc"} {
		got, err := game.ParseProofMethod(s)
		require.NoError(t, err, "ParseProofMethod(%q)", s)
		assert.Equal(t, game.ProofMethod(s), got)
	}
	_, err := game.ParseProofMethod("bluetooth")
	assert.Error(t, err)
}

// An area has no single tag to touch, so NFC cannot bound it; an object moves,
// so GPS proves nothing about it.
func TestSpaceKindAllowsMethod(t *testing.T) {
	cases := []struct {
		kind   game.SpaceKind
		method game.ProofMethod
		want   bool
	}{
		{game.SpaceKindPoint, game.ProofMethodGPS, true},
		{game.SpaceKindPoint, game.ProofMethodQR, true},
		{game.SpaceKindPoint, game.ProofMethodNFC, true},

		{game.SpaceKindArea, game.ProofMethodGPS, true},
		{game.SpaceKindArea, game.ProofMethodQR, true},
		{game.SpaceKindArea, game.ProofMethodNFC, false},

		{game.SpaceKindObject, game.ProofMethodGPS, false},
		{game.SpaceKindObject, game.ProofMethodQR, true},
		{game.SpaceKindObject, game.ProofMethodNFC, true},

		{game.SpaceKind("unknown"), game.ProofMethodQR, false},
		{game.SpaceKindPoint, game.ProofMethod("bluetooth"), false},
	}
	for _, c := range cases {
		got := game.SpaceKindAllowsMethod(c.kind, c.method)
		assert.Equal(t, c.want, got, "SpaceKindAllowsMethod(%q, %q)", c.kind, c.method)
	}
}

func TestProofMethodsForKind(t *testing.T) {
	assert.Equal(t,
		[]game.ProofMethod{game.ProofMethodGPS, game.ProofMethodQR, game.ProofMethodNFC},
		game.ProofMethodsForKind(game.SpaceKindPoint))
	assert.Equal(t,
		[]game.ProofMethod{game.ProofMethodGPS, game.ProofMethodQR},
		game.ProofMethodsForKind(game.SpaceKindArea))
	assert.Equal(t,
		[]game.ProofMethod{game.ProofMethodQR, game.ProofMethodNFC},
		game.ProofMethodsForKind(game.SpaceKindObject))
	assert.Nil(t, game.ProofMethodsForKind(game.SpaceKind("unknown")))
}

// --- SpaceGeometry ---

func closedRing() game.Ring {
	return game.Ring{
		game.NewPosition(-45.87, 170.50),
		game.NewPosition(-45.88, 170.51),
		game.NewPosition(-45.89, 170.49),
		game.NewPosition(-45.87, 170.50),
	}
}

// Getting the longitude-first convention backwards puts every space on the
// wrong side of the planet.
func TestPosition_Order(t *testing.T) {
	p := game.NewPosition(-45.8788, 170.5028)
	assert.InDelta(t, -45.8788, p.Lat(), 0.00001)
	assert.InDelta(t, 170.5028, p.Lng(), 0.00001)
	assert.InDelta(t, 170.5028, p[0], 0.00001, "GeoJSON stores longitude first")
	assert.InDelta(t, -45.8788, p[1], 0.00001, "GeoJSON stores latitude second")
}

func TestSpaceGeometry_IsZero(t *testing.T) {
	var nilGeom *game.SpaceGeometry
	assert.True(t, nilGeom.IsZero(), "nil geometry")
	assert.True(t, (&game.SpaceGeometry{}).IsZero(), "empty geometry")
	assert.True(t, (&game.SpaceGeometry{Type: game.GeometryPolygon}).IsZero(), "polygon with no rings")
	assert.False(t, game.NewPointGeometry(-45.87, 170.5, 0).IsZero())
	assert.False(t, game.NewPointGeometry(0, 0, 25).IsZero(), "a radius alone is still something")
	assert.False(t, (&game.SpaceGeometry{
		Type:  game.GeometryPolygon,
		Rings: []game.Ring{closedRing()},
	}).IsZero())
}

func TestSpaceGeometry_HasCoordinates(t *testing.T) {
	var nilGeom *game.SpaceGeometry
	assert.False(t, nilGeom.HasCoordinates())
	assert.False(t, game.NewPointGeometry(0, 0, 25).HasCoordinates(), "a radius alone is not a position")
	assert.True(t, game.NewPointGeometry(-45.87, 170.5, 0).HasCoordinates())
	assert.True(t, (&game.SpaceGeometry{
		Type:  game.GeometryPolygon,
		Rings: []game.Ring{closedRing()},
	}).HasCoordinates())
}

// The wire format has to be exactly what a map tool emits, so a draw control
// can round-trip it untouched.
func TestSpaceGeometry_MarshalsAsGeoJSON(t *testing.T) {
	data, err := json.Marshal(game.NewPointGeometry(-45.8788, 170.5028, 20))
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "Point", decoded["type"])
	assert.Equal(t, []any{170.5028, -45.8788}, decoded["coordinates"], "longitude first")
	assert.InDelta(t, 20.0, decoded["radius"], 0.001)
}

func TestSpaceGeometry_MarshalsPolygonAsGeoJSON(t *testing.T) {
	data, err := json.Marshal(&game.SpaceGeometry{
		Type:  game.GeometryPolygon,
		Rings: []game.Ring{closedRing()},
	})
	require.NoError(t, err)

	var decoded struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "Polygon", decoded.Type)
	require.Len(t, decoded.Coordinates, 1, "one outer ring")
	require.Len(t, decoded.Coordinates[0], 4, "closed ring repeats its first position")
	assert.InDelta(t, 170.50, decoded.Coordinates[0][0][0], 0.001, "longitude first")
}

func TestSpaceGeometry_ZeroMarshalsAsNull(t *testing.T) {
	data, err := json.Marshal(&game.SpaceGeometry{})
	require.NoError(t, err)
	assert.JSONEq(t, "null", string(data))
}

// The literal shape Mapbox Draw hands over, proving no translation layer is needed.
func TestSpaceGeometry_UnmarshalsMapboxOutput(t *testing.T) {
	raw := `{"type":"Polygon","coordinates":[[
		[170.5028,-45.8788],[170.5040,-45.8790],[170.5030,-45.8800],[170.5028,-45.8788]
	]]}`

	var g game.SpaceGeometry
	require.NoError(t, json.Unmarshal([]byte(raw), &g))

	assert.Equal(t, game.GeometryPolygon, g.Type)
	require.Len(t, g.Rings, 1)
	require.Len(t, g.Rings[0], 4)
	assert.InDelta(t, -45.8788, g.Rings[0][0].Lat(), 0.00001)
	assert.InDelta(t, 170.5028, g.Rings[0][0].Lng(), 0.00001)
	require.NoError(t, g.Validate())
}

func TestSpaceGeometry_UnmarshalNull(t *testing.T) {
	g := *game.NewPointGeometry(-45.87, 170.5, 10)
	require.NoError(t, json.Unmarshal([]byte("null"), &g))
	assert.Equal(t, game.GeometryPoint, g.Type, "null should leave the value untouched")
}

func TestSpaceGeometry_UnmarshalUnknownType(t *testing.T) {
	var g game.SpaceGeometry
	err := json.Unmarshal([]byte(`{"type":"LineString","coordinates":[[1,2],[3,4]]}`), &g)
	require.ErrorIs(t, err, game.ErrGeometryType)
}

func TestSpaceGeometry_Validate(t *testing.T) {
	cases := []struct {
		name    string
		geom    *game.SpaceGeometry
		wantErr error
	}{
		{name: "nil", geom: nil},
		{name: "empty", geom: &game.SpaceGeometry{}},
		{name: "valid point", geom: game.NewPointGeometry(-45.8788, 170.5028, 20)},
		{
			name: "valid polygon",
			geom: &game.SpaceGeometry{Type: game.GeometryPolygon, Rings: []game.Ring{closedRing()}},
		},
		{
			name:    "latitude out of range",
			geom:    game.NewPointGeometry(91, 170.5, 0),
			wantErr: game.ErrGeometryLatitude,
		},
		{
			name:    "longitude out of range",
			geom:    game.NewPointGeometry(-45.87, 181, 0),
			wantErr: game.ErrGeometryLongitude,
		},
		{
			name:    "negative radius",
			geom:    game.NewPointGeometry(-45.87, 170.5, -5),
			wantErr: game.ErrNegativeRadius,
		},
		{
			name:    "polygon with no rings",
			geom:    &game.SpaceGeometry{Type: game.GeometryPolygon, Rings: []game.Ring{{}}},
			wantErr: game.ErrRingTooShort,
		},
		{
			name: "ring too short",
			geom: &game.SpaceGeometry{Type: game.GeometryPolygon, Rings: []game.Ring{{
				game.NewPosition(-45.87, 170.50),
				game.NewPosition(-45.88, 170.51),
				game.NewPosition(-45.87, 170.50),
			}}},
			wantErr: game.ErrRingTooShort,
		},
		{
			name: "ring not closed",
			geom: &game.SpaceGeometry{Type: game.GeometryPolygon, Rings: []game.Ring{{
				game.NewPosition(-45.87, 170.50),
				game.NewPosition(-45.88, 170.51),
				game.NewPosition(-45.89, 170.49),
				game.NewPosition(-45.86, 170.52),
			}}},
			wantErr: game.ErrRingNotClosed,
		},
		{
			name: "position in ring out of range",
			geom: &game.SpaceGeometry{Type: game.GeometryPolygon, Rings: []game.Ring{{
				game.NewPosition(-45.87, 170.50),
				game.NewPosition(-95, 170.51),
				game.NewPosition(-45.89, 170.49),
				game.NewPosition(-45.87, 170.50),
			}}},
			wantErr: game.ErrGeometryLatitude,
		},
		{
			name:    "unknown type",
			geom:    &game.SpaceGeometry{Type: game.GeometryType("LineString"), Radius: 1},
			wantErr: game.ErrGeometryType,
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

func TestSpaceGeometry_ValueNilWhenEmpty(t *testing.T) {
	v, err := (&game.SpaceGeometry{}).Value()
	require.NoError(t, err)
	assert.Nil(t, v, "empty geometry should store as SQL NULL")
}

func TestSpaceGeometry_ValueScanRoundTrip(t *testing.T) {
	original := &game.SpaceGeometry{
		Type:  game.GeometryPolygon,
		Rings: []game.Ring{closedRing()},
	}

	v, err := original.Value()
	require.NoError(t, err)
	require.IsType(t, "", v)

	var restored game.SpaceGeometry
	require.NoError(t, restored.Scan(v))
	assert.Equal(t, *original, restored)

	// Scanning from []byte is the other shape drivers hand back.
	var fromBytes game.SpaceGeometry
	require.NoError(t, fromBytes.Scan([]byte(v.(string))))
	assert.Equal(t, *original, fromBytes)
}

func TestSpaceGeometry_ScanEdgeCases(t *testing.T) {
	var g game.SpaceGeometry
	require.NoError(t, g.Scan(nil), "nil should leave geometry untouched")
	assert.True(t, g.IsZero())

	require.NoError(t, g.Scan(""), "empty string should leave geometry untouched")
	assert.True(t, g.IsZero())

	assert.Error(t, g.Scan(42), "unsupported type should error")
}

// A short array would zero-fill into the fixed-size Position, landing the space
// on the equator without complaint.
func TestPosition_RejectsShortCoordinates(t *testing.T) {
	var p game.Position
	require.ErrorIs(t, json.Unmarshal([]byte(`[170.5028]`), &p), game.ErrPositionTooShort)
	require.ErrorIs(t, json.Unmarshal([]byte(`[]`), &p), game.ErrPositionTooShort)
}

// RFC 7946 allows a third element for altitude, which this does not use.
func TestPosition_DiscardsAltitude(t *testing.T) {
	var p game.Position
	require.NoError(t, json.Unmarshal([]byte(`[170.5028,-45.8788,12.5]`), &p))
	assert.InDelta(t, 170.5028, p.Lng(), 0.00001)
	assert.InDelta(t, -45.8788, p.Lat(), 0.00001)
}

func TestSpaceGeometry_RejectsShortCoordinates(t *testing.T) {
	var g game.SpaceGeometry
	require.ErrorIs(t,
		json.Unmarshal([]byte(`{"type":"Point","coordinates":[170.5028]}`), &g),
		game.ErrPositionTooShort)

	shortInRing := `{"type":"Polygon","coordinates":[[` +
		`[170.50,-45.87],[170.51],[170.49,-45.89],[170.50,-45.87]` +
		`]]}`
	var polygon game.SpaceGeometry
	require.ErrorIs(t, json.Unmarshal([]byte(shortInRing), &polygon), game.ErrPositionTooShort)
}
