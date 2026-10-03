package blocks

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const (
	// earthRadiusMetres is the mean radius, which is what haversine assumes.
	earthRadiusMetres = 6371000.0
	halfTurnDegrees   = 180.0
	// metresPerDegreeLat is constant; longitude shrinks with the cosine of latitude.
	metresPerDegreeLat = earthRadiusMetres * math.Pi / halfTurnDegrees
)

type GeometryType string

// The casing is deliberate: "Polygon" is GeoJSON's own spelling, "circle" is not
// GeoJSON at all. The spec has no circle, and a Point carrying a radius as a
// foreign member reads as a bare point to anything unaware of the convention.
const (
	GeometryCircle  GeometryType = "circle"
	GeometryPolygon GeometryType = "Polygon"
)

func (t GeometryType) known() bool {
	return t == GeometryCircle || t == GeometryPolygon
}

// Malformed geometry only. Whether a shape suits its use is the caller's call.
var (
	ErrGeometryType        = errors.New("geometry must be a circle or a Polygon")
	ErrGeometryLatitude    = errors.New("latitude must be between -90 and 90")
	ErrGeometryLongitude   = errors.New("longitude must be between -180 and 180")
	ErrCircleNeedsRadius   = errors.New("a circle needs a radius greater than zero")
	ErrPositionTooShort    = errors.New("a coordinate needs both a longitude and a latitude")
	ErrEmptyPolygon        = errors.New("a polygon needs at least one ring")
	ErrLinearRingTooShort  = errors.New("a polygon ring needs at least four positions, the last repeating the first")
	ErrLinearRingNotClosed = errors.New("a polygon ring must end where it starts")
)

const (
	// Three distinct corners plus the repeat of the first.
	minRingPositions = 4
	positionLength   = 2
)

// Position is a coordinate: longitude first, the reverse of how people say them.
// Use NewPosition and the accessors rather than indexing.
//
//nolint:recvcheck // the accessors read a value; UnmarshalJSON must take a pointer
type Position [2]float64

// NewPosition takes latitude first; the stored array is longitude first.
func NewPosition(lat, lng float64) Position {
	return Position{lng, lat}
}

func (p Position) Lng() float64 { return p[0] }

func (p Position) Lat() float64 { return p[1] }

// UnmarshalJSON checks length: decoding straight into the array would zero-fill
// a short coordinate, putting the latitude silently on the equator. A third
// element is altitude, unused here.
func (p *Position) UnmarshalJSON(data []byte) error {
	var raw []float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshalling position: %w", err)
	}
	if len(raw) < positionLength {
		return ErrPositionTooShort
	}
	p[0], p[1] = raw[0], raw[1]
	return nil
}

// LinearRing is a polygon boundary whose last position repeats the first.
type LinearRing []Position

// Geometry answers one question: is the player inside? A circle is a centre and
// a radius in metres; an area is a GeoJSON Polygon, kept verbatim so a map draw
// control round-trips it untouched. There is no bare point, because a position
// with no extent cannot answer that question.
type Geometry struct {
	Type        GeometryType
	Center      Position
	Radius      float64
	LinearRings []LinearRing
}

// NewCircle takes latitude first; the stored centre is longitude first.
func NewCircle(lat, lng, radius float64) *Geometry {
	return &Geometry{
		Type:   GeometryCircle,
		Center: NewPosition(lat, lng),
		Radius: radius,
	}
}

func NewPolygon(rings ...LinearRing) *Geometry {
	return &Geometry{
		Type:        GeometryPolygon,
		LinearRings: rings,
	}
}

// A circle and a polygon disagree on which field carries the shape, so the
// marshalling goes through here rather than struct tags.
type geometryWire struct {
	Type        GeometryType    `json:"type"`
	Center      *Position       `json:"center,omitempty"`
	Radius      float64         `json:"radius,omitempty"`
	Coordinates json.RawMessage `json:"coordinates,omitempty"`
}

func (g Geometry) MarshalJSON() ([]byte, error) {
	// Checked before IsZero, which reports true for an unrecognised type and
	// would write a broken shape away as null while reporting success.
	if g.Type != "" && !g.Type.known() {
		return nil, fmt.Errorf("marshalling geometry: %w", ErrGeometryType)
	}
	if g.IsZero() {
		return []byte("null"), nil
	}

	wire := geometryWire{Type: g.Type}
	switch g.Type {
	case GeometryCircle:
		centre := g.Center
		wire.Center = &centre
		wire.Radius = g.Radius
	case GeometryPolygon:
		raw, err := json.Marshal(g.LinearRings)
		if err != nil {
			return nil, fmt.Errorf("marshalling polygon rings: %w", err)
		}
		wire.Coordinates = raw
	default:
		return nil, fmt.Errorf("marshalling geometry: %w", ErrGeometryType)
	}
	return json.Marshal(wire)
}

func (g *Geometry) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var wire geometryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("unmarshalling geometry: %w", err)
	}

	*g = Geometry{Type: wire.Type, Radius: wire.Radius}
	switch wire.Type {
	case GeometryCircle:
		if wire.Center != nil {
			g.Center = *wire.Center
		}
		return nil
	case GeometryPolygon:
		if len(wire.Coordinates) == 0 {
			return nil
		}
		return json.Unmarshal(wire.Coordinates, &g.LinearRings)
	default:
		return fmt.Errorf("unmarshalling geometry: %w", ErrGeometryType)
	}
}

func (g *Geometry) IsZero() bool {
	if g == nil {
		return true
	}
	switch g.Type {
	case GeometryCircle:
		return g.Center == Position{} && g.Radius == 0
	case GeometryPolygon:
		return len(g.LinearRings) == 0
	}
	return true
}

func (g *Geometry) Validate() error {
	// An absent type is no geometry; an unrecognised one is a mistake. IsZero
	// treats both the same, so it cannot be used here.
	if g == nil || g.Type == "" {
		return nil
	}

	switch g.Type {
	case GeometryCircle:
		if g.Radius <= 0 {
			return ErrCircleNeedsRadius
		}
		return validatePosition(g.Center)
	case GeometryPolygon:
		return validateLinearRings(g.LinearRings)
	default:
		return ErrGeometryType
	}
}

func validateLinearRings(rings []LinearRing) error {
	if len(rings) == 0 {
		return ErrEmptyPolygon
	}
	for _, ring := range rings {
		if len(ring) < minRingPositions {
			return ErrLinearRingTooShort
		}
		if ring[0] != ring[len(ring)-1] {
			return ErrLinearRingNotClosed
		}
		for _, p := range ring {
			if err := validatePosition(p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Geometry) Contains(p Position) bool {
	if g == nil {
		return false
	}
	switch g.Type {
	case GeometryCircle:
		return g.Radius > 0 && MetresBetween(g.Center, p) <= g.Radius
	case GeometryPolygon:
		return ringsContain(g.LinearRings, p)
	}
	return false
}

// MetresBetween is the great-circle distance, which stays honest at any scale.
//
//nolint:mnd // the halves and the doubling are the haversine formula itself
func MetresBetween(a, b Position) float64 {
	lat1 := radians(a.Lat())
	lat2 := radians(b.Lat())
	halfDLat := (lat2 - lat1) / 2
	halfDLng := radians(b.Lng()-a.Lng()) / 2

	h := math.Sin(halfDLat)*math.Sin(halfDLat) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(halfDLng)*math.Sin(halfDLng)
	// Clamped because rounding can push h a hair above 1, where Asin is undefined.
	return 2 * earthRadiusMetres * math.Asin(math.Sqrt(math.Min(1, h)))
}

func radians(degrees float64) float64 {
	return degrees * math.Pi / halfTurnDegrees
}

// MetresOutside is how far the position lies beyond the geometry, and zero once
// inside it. A polygon measures to its nearest edge, so a shape reports a near
// miss the same way a circle does.
func (g *Geometry) MetresOutside(p Position) float64 {
	if g == nil || g.Contains(p) {
		return 0
	}
	switch g.Type {
	case GeometryCircle:
		return math.Max(0, MetresBetween(g.Center, p)-g.Radius)
	case GeometryPolygon:
		return metresToRings(g.LinearRings, p)
	}
	return 0
}

// metresToRings measures to the nearest edge of any ring. A hole counts, so a
// position in a courtyard is told how far it is to the building around it.
func metresToRings(rings []LinearRing, p Position) float64 {
	nearest := math.Inf(1)
	for _, ring := range rings {
		for i := 1; i < len(ring); i++ {
			if d := metresToSegment(p, ring[i-1], ring[i]); d < nearest {
				nearest = d
			}
		}
	}
	if math.IsInf(nearest, 1) {
		return 0
	}
	return nearest
}

// metresToSegment works in a plane centred on the position: over the span of a
// geofence a degree is a fixed number of metres, so the error stays far inside
// GPS noise, and this avoids a great-circle solve per edge.
func metresToSegment(p, a, b Position) float64 {
	lngScale := metresPerDegreeLat * math.Cos(radians(p.Lat()))

	px, py := 0.0, 0.0
	ax, ay := (a.Lng()-p.Lng())*lngScale, (a.Lat()-p.Lat())*metresPerDegreeLat
	bx, by := (b.Lng()-p.Lng())*lngScale, (b.Lat()-p.Lat())*metresPerDegreeLat

	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}

	// How far along the edge the closest point sits, clamped to its ends.
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// GeoJSON puts the outer boundary first and treats any later ring as a hole, so
// a position inside a hole is outside the polygon.
func ringsContain(rings []LinearRing, p Position) bool {
	if len(rings) == 0 || !ringContains(rings[0], p) {
		return false
	}
	for _, hole := range rings[1:] {
		if ringContains(hole, p) {
			return false
		}
	}
	return true
}

// ringContains is even-odd ray casting, treating longitude and latitude as plane
// coordinates. That is wrong near the poles and across the antimeridian, but at
// the scale a geofence covers the error stays far inside GPS noise.
func ringContains(ring LinearRing, p Position) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		xi, yi := ring[i].Lng(), ring[i].Lat()
		xj, yj := ring[j].Lng(), ring[j].Lat()

		if (yi > p.Lat()) == (yj > p.Lat()) {
			continue
		}
		if p.Lng() < (xj-xi)*(p.Lat()-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

func validatePosition(p Position) error {
	if p.Lat() < -90 || p.Lat() > 90 {
		return ErrGeometryLatitude
	}
	if p.Lng() < -180 || p.Lng() > 180 {
		return ErrGeometryLongitude
	}
	return nil
}

// Value stores empty geometry as SQL NULL rather than an empty object. It defers
// to MarshalJSON rather than testing IsZero itself, so both write paths agree on
// what counts as nothing and neither can swallow an unrecognised type.
func (g *Geometry) Value() (driver.Value, error) {
	data, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("marshalling Geometry: %w", err)
	}
	if string(data) == "null" {
		return nil, nil //nolint:nilnil // nil driver.Value = SQL NULL; nil error = no failure
	}
	return string(data), nil
}

func (g *Geometry) Scan(src any) error {
	if src == nil {
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into Geometry", src)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, g)
}
