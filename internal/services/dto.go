package services

// LocationData is the data required to update a new location. Blank
// fields are ignored, with the exception of Clues and ClueIDs which
// are always required.
type LocationUpdateData struct {
	Name      string
	Latitude  float64
	Longitude float64
	Points    int
}
