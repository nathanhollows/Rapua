package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type StrArray []string

type NavigationMode int
type NavigationMethod int
type GameStatus int
type Provider string

type NavigationModes []NavigationMode
type NavigationMethods []NavigationMethod
type GameStatuses []GameStatus

const (
	RandomNav NavigationMode = iota
	FreeRoamNav
	OrderedNav
)

const (
	ShowMap NavigationMethod = iota
	ShowMapAndNames
	ShowNames
	ShowClues
)

const (
	Scheduled GameStatus = iota
	Active
	Closed
)

const (
	ProviderGoogle Provider = "google"
	ProviderEmail  Provider = ""
)

// Value converts StrArray to a JSON string for database storage.
func (s StrArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	bytes, err := json.Marshal(s)
	return string(bytes), err
}

// Scan converts a database JSON string back into a StrArray.
func (s *StrArray) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("failed to scan StrArray: expected string, got %T", value)
	}

	err := json.Unmarshal([]byte(str), s)
	return err
}

// GetNavigationModes returns a list of navigation modes.
func GetNavigationModes() NavigationModes {
	return []NavigationMode{RandomNav, FreeRoamNav, OrderedNav}
}

// GetNavigationMethods returns a list of navigation methods.
func GetNavigationMethods() NavigationMethods {
	return []NavigationMethod{ShowMap, ShowMapAndNames, ShowNames, ShowClues}
}

// GetGameStatuses returns a list of game statuses.
func GetGameStatuses() GameStatuses {
	return []GameStatus{Scheduled, Active, Closed}
}

// String returns the string representation of the NavigationMode.
func (n NavigationMode) String() string {
	return [...]string{"Randomised Route", "Open Exploration", "Guided Path"}[n]
}

// String returns the string representation of the NavigationMethod.
func (n NavigationMethod) String() string {
	return [...]string{"Map Only", "Labelled Map", "Location List", "Clue-Based"}[n]
}

// String returns the string representation of the GameStatus.
func (g GameStatus) String() string {
	return [...]string{"Scheduled", "Active", "Closed"}[g]
}

// Description returns the description of the NavigationMode.
func (n NavigationMode) Description() string {
	return [...]string{
		"The game will randomly select locations for players to visit. Good for large groups as it disperses players.",
		"Players can visit locations in any order. This mode shows all locations and is good for exploration.",
		"Players must visit locations in a specific order. Good for narrative experiences.",
	}[n]
}

// Description returns the description of the NavigationMethod.
func (n NavigationMethod) Description() string {
	return [...]string{
		"Players are shown a map.",
		"Players are shown a map with location names.",
		"Players are shown a list of locations by name.",
		"Players are shown clues but not the location or name.",
	}[n]
}

// Description returns the description of the GameStatus.
func (g GameStatus) Description() string {
	return [...]string{
		"The game is scheduled but not yet active.",
		"The game is active and players can participate.",
		"The game is closed and players cannot participate.",
	}[g]
}

// Parse NavigationMode.
func ParseNavigationMode(s string) (NavigationMode, error) {
	switch s {
	case "Random", "Randomised Route":
		return RandomNav, nil
	case "Free Roam", "Open Exploration":
		return FreeRoamNav, nil
	case "Ordered", "Guided Path":
		return OrderedNav, nil
	default:
		return 0, errors.New("invalid NavigationMode")
	}
}

// Parse NavigationMethod.
func ParseNavigationMethod(s string) (NavigationMethod, error) {
	switch s {
	case "Show Map", "Map Only":
		return ShowMap, nil
	case "Show Map and Names", "Labelled Map":
		return ShowMapAndNames, nil
	case "Show Location Names", "Location List":
		return ShowNames, nil
	case "Show Clues", "Clue-Based":
		return ShowClues, nil
	default:
		return ShowMap, errors.New("invalid NavigationMethod")
	}
}

// Parse GameStatus.
func ParseGameStatus(s string) (GameStatus, error) {
	switch s {
	case "Scheduled":
		return Scheduled, nil
	case "Active":
		return Active, nil
	case "Closed":
		return Closed, nil
	default:
		return 0, errors.New("invalid GameStatus")
	}
}
