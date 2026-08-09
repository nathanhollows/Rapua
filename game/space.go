package game

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type SpaceKind string

const (
	SpaceKindPoint  SpaceKind = "point"
	SpaceKindArea   SpaceKind = "area"
	SpaceKindObject SpaceKind = "object"
)

type ProofMethod string

const (
	ProofMethodGPS ProofMethod = "gps"
	ProofMethodQR  ProofMethod = "qr"
	ProofMethodNFC ProofMethod = "nfc"
)

func (k SpaceKind) Valid() bool {
	switch k {
	case SpaceKindPoint, SpaceKindArea, SpaceKindObject:
		return true
	}
	return false
}

func (k SpaceKind) String() string {
	switch k {
	case SpaceKindPoint:
		return "Point"
	case SpaceKindArea:
		return "Area"
	case SpaceKindObject:
		return "Object"
	default:
		return string(k)
	}
}

func (k SpaceKind) Description() string {
	switch k {
	case SpaceKindPoint:
		return "A single spot players stand at, such as a bench or a sign. Has coordinates and a radius."
	case SpaceKindArea:
		return "A region players move around in, such as a gallery or a paddock. Has a boundary or a radius."
	case SpaceKindObject:
		return "A movable thing players find, such as a specimen box or a prop. Has no fixed coordinates."
	default:
		return ""
	}
}

func ParseSpaceKind(s string) (SpaceKind, error) {
	k := SpaceKind(s)
	if !k.Valid() {
		return "", errors.New("invalid SpaceKind")
	}
	return k, nil
}

func (m ProofMethod) Valid() bool {
	switch m {
	case ProofMethodGPS, ProofMethodQR, ProofMethodNFC:
		return true
	}
	return false
}

func (m ProofMethod) String() string {
	switch m {
	case ProofMethodGPS:
		return "GPS"
	case ProofMethodQR:
		return "QR code"
	case ProofMethodNFC:
		return "NFC tag"
	default:
		return string(m)
	}
}

func ParseProofMethod(s string) (ProofMethod, error) {
	m := ProofMethod(s)
	if !m.Valid() {
		return "", errors.New("invalid ProofMethod")
	}
	return m, nil
}

func ProofMethodsForKind(kind SpaceKind) []ProofMethod {
	switch kind {
	case SpaceKindPoint:
		return []ProofMethod{ProofMethodGPS, ProofMethodQR, ProofMethodNFC}
	case SpaceKindArea:
		// An area has no single tag to touch, so NFC cannot bound it.
		return []ProofMethod{ProofMethodGPS, ProofMethodQR}
	case SpaceKindObject:
		// An object moves, so its coordinates prove nothing.
		return []ProofMethod{ProofMethodQR, ProofMethodNFC}
	}
	return nil
}

func SpaceKindAllowsMethod(kind SpaceKind, method ProofMethod) bool {
	for _, m := range ProofMethodsForKind(kind) {
		if m == method {
			return true
		}
	}
	return false
}

type GeometryType string

const (
	GeometryPoint   GeometryType = "Point"
	GeometryPolygon GeometryType = "Polygon"
)

// Malformed GeoJSON only. Whether a shape suits a space's kind is the caller's call.
var (
	ErrGeometryType      = errors.New("geometry must be a Point or a Polygon")
	ErrGeometryLatitude  = errors.New("latitude must be between -90 and 90")
	ErrGeometryLongitude = errors.New("longitude must be between -180 and 180")
	ErrNegativeRadius    = errors.New("radius cannot be negative")
	ErrPositionTooShort  = errors.New("a coordinate needs both a longitude and a latitude")
	ErrEmptyPolygon      = errors.New("a polygon needs at least one ring")
	ErrRingTooShort      = errors.New("a polygon ring needs at least four positions, the last repeating the first")
	ErrRingNotClosed     = errors.New("a polygon ring must end where it starts")
)

const (
	// Three distinct corners plus the repeat of the first.
	minRingPositions = 4
	positionLength   = 2
)

// Position is a GeoJSON coordinate: longitude first, the reverse of how people
// say them. Use NewPosition and the accessors rather than indexing.
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

// Ring is one boundary of a polygon. GeoJSON requires the last position to repeat the first.
type Ring []Position

// SpaceGeometry is stored as a GeoJSON geometry so a map draw control can
// round-trip it untouched. GeoJSON has no circle primitive, so Radius rides
// along as a foreign member, which RFC 7946 §6.1 permits and Mapbox expects.
type SpaceGeometry struct {
	Type   GeometryType
	Point  Position
	Rings  []Ring
	Radius float64
}

func NewPointGeometry(lat, lng, radius float64) *SpaceGeometry {
	return &SpaceGeometry{
		Type:   GeometryPoint,
		Point:  NewPosition(lat, lng),
		Radius: radius,
	}
}

// Coordinates change Go type with the discriminator, so marshalling goes
// through here rather than struct tags.
type geometryWire struct {
	Type        GeometryType    `json:"type"`
	Coordinates json.RawMessage `json:"coordinates,omitempty"`
	Radius      float64         `json:"radius,omitempty"`
}

func (g SpaceGeometry) MarshalJSON() ([]byte, error) {
	if g.IsZero() {
		return []byte("null"), nil
	}

	var coords any
	switch g.Type {
	case GeometryPoint:
		coords = g.Point
	case GeometryPolygon:
		coords = g.Rings
	default:
		return nil, fmt.Errorf("marshalling geometry: %w", ErrGeometryType)
	}

	raw, err := json.Marshal(coords)
	if err != nil {
		return nil, fmt.Errorf("marshalling geometry coordinates: %w", err)
	}
	return json.Marshal(geometryWire{Type: g.Type, Coordinates: raw, Radius: g.Radius})
}

func (g *SpaceGeometry) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var wire geometryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("unmarshalling geometry: %w", err)
	}

	*g = SpaceGeometry{Type: wire.Type, Radius: wire.Radius}
	if len(wire.Coordinates) == 0 {
		return nil
	}

	switch wire.Type {
	case GeometryPoint:
		return json.Unmarshal(wire.Coordinates, &g.Point)
	case GeometryPolygon:
		return json.Unmarshal(wire.Coordinates, &g.Rings)
	default:
		return fmt.Errorf("unmarshalling geometry: %w", ErrGeometryType)
	}
}

func (g *SpaceGeometry) IsZero() bool {
	if g == nil {
		return true
	}
	switch g.Type {
	case GeometryPoint:
		return g.Point == Position{} && g.Radius == 0
	case GeometryPolygon:
		return len(g.Rings) == 0
	}
	return true
}

func (g *SpaceGeometry) HasCoordinates() bool {
	if g == nil {
		return false
	}
	switch g.Type {
	case GeometryPoint:
		return g.Point != Position{}
	case GeometryPolygon:
		return len(g.Rings) > 0 && len(g.Rings[0]) > 0
	}
	return false
}

func (g *SpaceGeometry) Validate() error {
	// An absent type is no geometry; an unrecognised one is a mistake. IsZero
	// treats both the same, so it cannot be used here.
	if g == nil || g.Type == "" {
		return nil
	}

	if g.Radius < 0 {
		return ErrNegativeRadius
	}

	switch g.Type {
	case GeometryPoint:
		return validatePosition(g.Point)
	case GeometryPolygon:
		return validateRings(g.Rings)
	default:
		return ErrGeometryType
	}
}

func validateRings(rings []Ring) error {
	if len(rings) == 0 {
		return ErrEmptyPolygon
	}
	for _, ring := range rings {
		if len(ring) < minRingPositions {
			return ErrRingTooShort
		}
		if ring[0] != ring[len(ring)-1] {
			return ErrRingNotClosed
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

// Value stores empty geometry as SQL NULL rather than an empty object.
func (g *SpaceGeometry) Value() (driver.Value, error) {
	if g.IsZero() {
		return nil, nil //nolint:nilnil // nil driver.Value = SQL NULL; nil error = no failure
	}
	data, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("marshalling SpaceGeometry: %w", err)
	}
	return string(data), nil
}

func (g *SpaceGeometry) Scan(src any) error {
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
		return fmt.Errorf("cannot scan %T into SpaceGeometry", src)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, g)
}
