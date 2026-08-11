package blocks

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
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
