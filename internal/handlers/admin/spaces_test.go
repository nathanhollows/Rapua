package admin //nolint:testpackage // parseSpaceForm and spaceErrorMessage are unexported

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A closed ring, in the shape a map tool exports.
func squareBoundary() string {
	return `{"type":"Polygon","coordinates":[[` +
		`[170.50,-45.87],[170.51,-45.88],[170.49,-45.89],[170.50,-45.87]` +
		`]]}`
}

func spaceFormRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/admin/spaces", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestParseSpaceForm_Point(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":   {"Totara Grove"},
		"slug":   {" the-grove "},
		"kind":   {"point"},
		"lat":    {"-45.8788"},
		"lng":    {"170.5028"},
		"radius": {"20"},
	}))
	require.NoError(t, err)

	assert.Equal(t, "Totara Grove", in.Name)
	assert.Equal(t, "the-grove", in.Slug)
	assert.Equal(t, game.SpaceKindPoint, in.Kind)
	require.NotNil(t, in.Geometry)
	assert.Equal(t, game.GeometryPoint, in.Geometry.Type)
	assert.InDelta(t, -45.8788, in.Geometry.Point.Lat(), 0.00001)
	assert.InDelta(t, 170.5028, in.Geometry.Point.Lng(), 0.00001)
	assert.InDelta(t, 20, in.Geometry.Radius, 0.001)
	assert.False(t, in.Mobile)
}

// An object has no fixed position, so any coordinates left in the form from a
// previous kind must not survive the parse.
func TestParseSpaceForm_ObjectDropsCoordinates(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":   {"Specimen box"},
		"kind":   {"object"},
		"lat":    {"-45.8788"},
		"lng":    {"170.5028"},
		"mobile": {"on"},
	}))
	require.NoError(t, err)

	assert.Equal(t, game.SpaceKindObject, in.Kind)
	assert.Nil(t, in.Geometry, "an object carries no geometry")
	assert.True(t, in.Mobile)
}

func TestParseSpaceForm_PointIgnoresBoundary(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Totara Grove"},
		"kind":     {"point"},
		"lat":      {"-45.8788"},
		"lng":      {"170.5028"},
		"boundary": {squareBoundary()},
	}))
	require.NoError(t, err)
	require.NotNil(t, in.Geometry)
	assert.Equal(t, game.GeometryPoint, in.Geometry.Type)
}

func TestParseSpaceForm_AreaWithBoundary(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name": {"Aviary"},
		"kind": {"area"},
		"boundary": {`{"type":"Polygon","coordinates":[[
			[170.5028,-45.8788],[170.5040,-45.8790],[170.5030,-45.8800],[170.5028,-45.8788]
		]]}`},
	}))
	require.NoError(t, err)

	require.NotNil(t, in.Geometry)
	assert.Equal(t, game.GeometryPolygon, in.Geometry.Type)
	require.Len(t, in.Geometry.Rings, 1)
	require.Len(t, in.Geometry.Rings[0], 4)
	assert.InDelta(t, -45.8788, in.Geometry.Rings[0][0].Lat(), 0.00001)
	assert.InDelta(t, 170.5028, in.Geometry.Rings[0][0].Lng(), 0.00001)
	require.NoError(t, services.ValidateSpaceInput(in))
}

func TestParseSpaceForm_BoundaryBeatsCentre(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"lat":      {"-45.8788"},
		"lng":      {"170.5028"},
		"radius":   {"50"},
		"boundary": {squareBoundary()},
	}))
	require.NoError(t, err)
	assert.Equal(t, game.GeometryPolygon, in.Geometry.Type)
}

func TestParseSpaceForm_AreaFallsBackToCentre(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"lat":      {"-45.8788"},
		"lng":      {"170.5028"},
		"radius":   {"50"},
		"boundary": {"   "},
	}))
	require.NoError(t, err)
	require.NotNil(t, in.Geometry)
	assert.Equal(t, game.GeometryPoint, in.Geometry.Type)
	assert.InDelta(t, 50, in.Geometry.Radius, 0.001)
}

func TestParseSpaceForm_BadBoundary(t *testing.T) {
	for name, boundary := range map[string]string{
		"not json":       "{nope",
		"not a geometry": `{"hello":"world"}`,
		"wrong type":     `{"type":"LineString","coordinates":[[1,2],[3,4]]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseSpaceForm(spaceFormRequest(url.Values{
				"name":     {"Aviary"},
				"kind":     {"area"},
				"boundary": {boundary},
			}))
			require.ErrorIs(t, err, errBoundaryJSON)
		})
	}
}

// A latitude with no longitude used to sail through as (lat, 0), a few thousand
// kilometres off the coast of Africa.
func TestParseSpaceForm_RejectsHalfFilledCoordinates(t *testing.T) {
	for name, form := range map[string]url.Values{
		"longitude missing": {"name": {"Totara"}, "kind": {"point"}, "lat": {"-45.8788"}, "lng": {""}},
		"latitude missing":  {"name": {"Totara"}, "kind": {"point"}, "lat": {""}, "lng": {"170.5028"}},
		"radius only":       {"name": {"Totara"}, "kind": {"point"}, "radius": {"25"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseSpaceForm(spaceFormRequest(form))
			require.ErrorIs(t, err, errCoordinatePair)
		})
	}
}

// Garbage in a coordinate box is a mistake, not a zero.
func TestParseSpaceForm_RejectsUnparseableNumbers(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr error
	}{
		{
			name:    "latitude is not a number",
			form:    url.Values{"name": {"Totara"}, "kind": {"point"}, "lat": {"north"}, "lng": {"170.5"}},
			wantErr: errCoordinateNumber,
		},
		{
			name:    "longitude is not a number",
			form:    url.Values{"name": {"Totara"}, "kind": {"point"}, "lat": {"-45.87"}, "lng": {"east"}},
			wantErr: errCoordinateNumber,
		},
		{
			name: "radius is not a number",
			form: url.Values{
				"name": {"Totara"}, "kind": {"point"},
				"lat": {"-45.87"}, "lng": {"170.5"}, "radius": {"wide"},
			},
			wantErr: errRadiusNumber,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseSpaceForm(spaceFormRequest(c.form))
			require.ErrorIs(t, err, c.wantErr)
		})
	}
}

// Both boxes blank is a space with no position yet, which the service judges
// against the kind — not a parse failure.
func TestParseSpaceForm_BlankCoordinatesGiveNoGeometry(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name": {"Totara Grove"},
		"kind": {"point"},
		"lat":  {""},
		"lng":  {""},
	}))
	require.NoError(t, err)
	assert.Nil(t, in.Geometry)
	require.ErrorIs(t, services.ValidateSpaceInput(in), services.ErrPointNeedsCoords)
}

func TestSpaceErrorMessage(t *testing.T) {
	assert.Equal(t, services.ErrPointNeedsCoords.Error(), spaceErrorMessage(services.ErrPointNeedsCoords))
	assert.Equal(t, services.ErrSlugTaken.Error(), spaceErrorMessage(services.ErrSlugTaken))
	assert.Equal(t, services.ErrPayloadTaken.Error(), spaceErrorMessage(services.ErrPayloadTaken))
	assert.Equal(t, game.ErrRingNotClosed.Error(), spaceErrorMessage(game.ErrRingNotClosed))
	assert.Equal(t, errCoordinatePair.Error(), spaceErrorMessage(errCoordinatePair))
	assert.Equal(t, "Error saving space", spaceErrorMessage(assert.AnError),
		"unexpected errors should not leak internals to the author")
}

// Pasting a Point is a centre, so the radius box must still be read — otherwise
// the author is told an extent is missing while looking at both filled in.
func TestParseSpaceForm_PastedPointBoundaryKeepsRadius(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"radius":   {"50"},
		"boundary": {`{"type":"Point","coordinates":[170.5028,-45.8788]}`},
	}))
	require.NoError(t, err)

	require.NotNil(t, in.Geometry)
	assert.Equal(t, game.GeometryPoint, in.Geometry.Type)
	assert.InDelta(t, 50, in.Geometry.Radius, 0.001)
	require.NoError(t, services.ValidateSpaceInput(in), "a pasted centre plus a radius is a valid area")
}

// A radius in the pasted geometry came from the same export as the coordinates.
func TestParseSpaceForm_PastedPointKeepsItsOwnRadius(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"radius":   {"50"},
		"boundary": {`{"type":"Point","coordinates":[170.5028,-45.8788],"radius":120}`},
	}))
	require.NoError(t, err)
	assert.InDelta(t, 120, in.Geometry.Radius, 0.001)
}

// A pasted polygon has an extent of its own, so the radius box is irrelevant.
func TestParseSpaceForm_PastedPolygonIgnoresRadius(t *testing.T) {
	in, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"radius":   {"50"},
		"boundary": {squareBoundary()},
	}))
	require.NoError(t, err)
	assert.Equal(t, game.GeometryPolygon, in.Geometry.Type)
	assert.Zero(t, in.Geometry.Radius)
}

func TestParseSpaceForm_PastedPointRejectsBadRadius(t *testing.T) {
	_, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"radius":   {"wide"},
		"boundary": {`{"type":"Point","coordinates":[170.5028,-45.8788]}`},
	}))
	require.ErrorIs(t, err, errRadiusNumber)
}

func TestParseSpaceForm_RejectsShortBoundaryCoordinates(t *testing.T) {
	_, err := parseSpaceForm(spaceFormRequest(url.Values{
		"name":     {"Aviary"},
		"kind":     {"area"},
		"boundary": {`{"type":"Point","coordinates":[170.5028]}`},
	}))
	require.ErrorIs(t, err, errBoundaryJSON)
}
