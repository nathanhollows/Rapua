package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	geofenceBlockType = "geofence"
)

// DefaultMaxAccuracy is the widest fix still worth testing when an author sets no
// limit. Good outdoor GPS lands within 3 to 10 metres and a poor fix under cover
// within 40. Beyond this a reading comes from wifi or an IP address and is not a
// position at all.
//
// Exported because the browser needs the same number to decide when a fix is
// worth posting, and two copies would drift.
const DefaultMaxAccuracy = 50.0

type GeofenceBlock struct {
	BaseBlock
	Prompt string    `json:"prompt"`
	Area   *Geometry `json:"area"`
	// MaxAccuracy is the widest margin, in metres, still worth testing. A reading
	// vaguer than this is refused rather than guessed at.
	MaxAccuracy float64 `json:"max_accuracy,omitempty"`
	// MapMode controls what the player sees while checking their position. The
	// zero value keeps today's behaviour: a bare button, with the area withheld
	// so finding it is part of the challenge.
	MapMode GeofenceMapMode `json:"map_mode,omitempty"`
}

// GeofenceMapMode is the player-facing map an author has opted into, if any.
type GeofenceMapMode string

const (
	// GeofenceMapButton is the zero value: no map, area withheld.
	GeofenceMapButton GeofenceMapMode = "button"
	// GeofenceMapShow draws the player's own live position, area withheld.
	GeofenceMapShow GeofenceMapMode = "map"
	// GeofenceMapArea draws the player's live position and the target area.
	GeofenceMapArea GeofenceMapMode = "map_area"
)

func (m GeofenceMapMode) known() bool {
	return m == GeofenceMapButton || m == GeofenceMapShow || m == GeofenceMapArea
}

type geofenceBlockData struct {
	Attempts int     `json:"attempts"`
	Lat      float64 `json:"lat,omitempty"`
	Lng      float64 `json:"lng,omitempty"`
	Accuracy float64 `json:"accuracy,omitempty"`
	// MetresAway is kept for the near miss: it separates standing at the wrong
	// place from standing just outside the right one.
	MetresAway float64 `json:"metres_away,omitempty"`
}

func (b *GeofenceBlock) GetID() string      { return b.ID }
func (b *GeofenceBlock) GetType() string    { return geofenceBlockType }
func (b *GeofenceBlock) GetOwnerID() string { return b.OwnerID }
func (b *GeofenceBlock) GetName() string    { return "Location check" }
func (b *GeofenceBlock) GetDescription() string {
	return "Players must be standing in a place for this to pass."
}
func (b *GeofenceBlock) GetOrder() int  { return b.Order }
func (b *GeofenceBlock) GetPoints() int { return b.Points }
func (b *GeofenceBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-map-pin"><path d="M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0"/><circle cx="12" cy="10" r="3"/></svg>`
}

func (b *GeofenceBlock) GetAdminData() interface{} {
	return &b
}

func (b *GeofenceBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

func (b *GeofenceBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *GeofenceBlock) UpdateBlockData(input map[string][]string) error {
	if input["points"] != nil {
		points, err := strconv.Atoi(input["points"][0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	}

	if input["prompt"] != nil {
		b.Prompt = input["prompt"][0]
	}

	if raw := firstValue(input, "area"); raw != "" {
		var area Geometry
		if err := json.Unmarshal([]byte(raw), &area); err != nil {
			return fmt.Errorf("area must be a circle or a GeoJSON polygon: %w", err)
		}
		if err := area.Validate(); err != nil {
			return err
		}
		b.Area = &area
	}

	if raw := firstValue(input, "max_accuracy"); raw != "" {
		accuracy, err := strconv.ParseFloat(raw, 64)
		if err != nil || accuracy <= 0 {
			return errors.New("max accuracy must be a number of metres above zero")
		}
		b.MaxAccuracy = accuracy
	}

	if raw := firstValue(input, "map_mode"); raw != "" {
		mode := GeofenceMapMode(raw)
		if !mode.known() {
			return errors.New("map mode must be button, map, or map_area")
		}
		b.MapMode = mode
	}
	return nil
}

func (b *GeofenceBlock) ToYAML() map[string]any {
	m := map[string]any{}
	if b.Prompt != "" {
		m["prompt"] = b.Prompt
	}
	if !b.Area.IsZero() {
		m["area"] = b.Area
	}
	if b.MaxAccuracy > 0 {
		m["max_accuracy"] = b.MaxAccuracy
	}
	if mode := b.mapMode(); mode != GeofenceMapButton {
		m["map_mode"] = string(mode)
	}
	return m
}

func (b *GeofenceBlock) SupportsVariableSets() bool { return true }
func (b *GeofenceBlock) RequiresValidation() bool   { return true }

// maxAccuracy defaults rather than failing closed, so an imported block without
// one still tests positions instead of refusing every reading.
func (b *GeofenceBlock) maxAccuracy() float64 {
	if b.MaxAccuracy > 0 {
		return b.MaxAccuracy
	}
	return DefaultMaxAccuracy
}

// mapMode defaults an unset or unrecognised value to the button-only mode, so a
// block predating this field, or one written by an older version, still hides
// the area rather than guessing it should be shown.
func (b *GeofenceBlock) mapMode() GeofenceMapMode {
	if b.MapMode.known() {
		return b.MapMode
	}
	return GeofenceMapButton
}

func (b *GeofenceBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	if b.Area.IsZero() {
		return state, errors.New("this block has no area set")
	}

	position, accuracy, err := reportedPosition(input)
	if err != nil {
		return state, err
	}
	if accuracy > b.maxAccuracy() {
		return state, fmt.Errorf(
			"your position is only accurate to about %.0fm, which is too vague to check", accuracy)
	}

	data := geofenceBlockData{}
	if state.GetPlayerData() != nil {
		if parseErr := json.Unmarshal(state.GetPlayerData(), &data); parseErr != nil {
			return state, fmt.Errorf("parse player data: %w", parseErr)
		}
	}

	data.Attempts++
	data.Lat, data.Lng, data.Accuracy = position.Lat(), position.Lng(), accuracy
	data.MetresAway = b.Area.MetresOutside(position)

	playerData, err := json.Marshal(data)
	if err != nil {
		return state, errors.New("error saving player data")
	}
	state.SetPlayerData(playerData)

	if !b.Area.Contains(position) {
		return state, nil
	}

	state.SetComplete(true)
	state.SetPointsAwarded(b.Points)
	return state, nil
}

// reportedPosition reads the fix the browser handed over. Accuracy is required:
// without it there is no way to tell a rooftop-precise reading from a guess at
// the city, and treating those alike is how a player passes from home.
func reportedPosition(input map[string][]string) (Position, float64, error) {
	lat, latErr := strconv.ParseFloat(firstValue(input, "lat"), 64)
	lng, lngErr := strconv.ParseFloat(firstValue(input, "lng"), 64)
	if latErr != nil || lngErr != nil {
		return Position{}, 0, errors.New("we could not read your position")
	}

	position := NewPosition(lat, lng)
	if err := validatePosition(position); err != nil {
		return Position{}, 0, err
	}

	accuracy, accErr := strconv.ParseFloat(firstValue(input, "accuracy"), 64)
	if accErr != nil || accuracy < 0 {
		return Position{}, 0, errors.New("we could not tell how accurate your position is")
	}
	return position, accuracy, nil
}

func firstValue(input map[string][]string, key string) string {
	if v := input[key]; len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}
