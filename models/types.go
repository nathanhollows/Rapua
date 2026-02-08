package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

//nolint:recvcheck // Value() requires value receiver, Scan() requires pointer receiver per database/sql interface
type StrArray []string

type RouteStrategy int
type NavigationDisplayMode int
type GameStatus int
type Provider string

type RouteStrategies []RouteStrategy
type NavigationDisplayModes []NavigationDisplayMode
type GameStatuses []GameStatus

const (
	RouteStrategyRandom RouteStrategy = iota
	RouteStrategyFreeRoam
	RouteStrategyOrdered
	RouteStrategySecret // Locations that may be accessed out of sequence
)

const (
	NavigationDisplayMap NavigationDisplayMode = iota
	NavigationDisplayMapAndNames
	NavigationDisplayNames
	NavigationDisplayClues  // Deprecated
	NavigationDisplayCustom // For Block content
	NavigationDisplayTasks  // Task checklist with completion tracking
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
func (s *StrArray) Scan(value any) error {
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

// GetRouteStrategies returns a list of navigation modes.
func GetRouteStrategies() RouteStrategies {
	return []RouteStrategy{RouteStrategyOrdered, RouteStrategyFreeRoam, RouteStrategyRandom, RouteStrategySecret}
}

// GetNavigationDisplayModes returns a list of navigation methods.
func GetNavigationDisplayModes() NavigationDisplayModes {
	return []NavigationDisplayMode{
		NavigationDisplayMap,
		NavigationDisplayMapAndNames,
		NavigationDisplayNames,
		NavigationDisplayCustom,
		NavigationDisplayTasks,
	}
}

// GetGameStatuses returns a list of game statuses.
func GetGameStatuses() GameStatuses {
	return []GameStatus{Scheduled, Active, Closed}
}

// String returns the string representation of the RouteStrategy.
func (n RouteStrategy) String() string {
	return [...]string{"Randomised Route", "Open Exploration", "Guided Path", "Secret"}[n]
}

// String returns the string representation of the NavigationDisplayMode.
func (n NavigationDisplayMode) String() string {
	return [...]string{"Map Only", "Labelled Map", "Location List", "Clue-Based", "Custom Clues", "Tasks"}[n]
}

// String returns the string representation of the GameStatus.
func (g GameStatus) String() string {
	return [...]string{"Scheduled", "Active", "Closed"}[g]
}

// Description returns the description of the RouteStrategy.
func (n RouteStrategy) Description() string {
	return [...]string{
		"The game will randomly select locations for players to visit. Good for large groups as it disperses players.",
		"Players can visit locations in any order. This mode shows all locations and is good for exploration.",
		"Players must visit locations in a specific order. Good for narrative experiences.",
		"Locations that may be accessed out of sequence. These locations are never explicitly shown to players.",
	}[n]
}

// Description returns the description of the NavigationDisplayMode.
func (n NavigationDisplayMode) Description() string {
	return [...]string{
		"Players are shown a map.",
		"Players are shown a map with location names.",
		"Players are shown a list of locations by name.",
		"Players are shown clues but not the location or name.", // Deprecated
		"Players are shown custom content, e.g., randomised clues or images, using the block builder.",
		"Players see a checklist, like a scavenger hunt, with completion tracking.",
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

// ParseRouteStrategy returns a RouteStrategy from a string.
func ParseRouteStrategy(s string) (RouteStrategy, error) {
	switch s {
	case "Random", "Randomised Route":
		return RouteStrategyRandom, nil
	case "Free Roam", "Open Exploration":
		return RouteStrategyFreeRoam, nil
	case "Ordered", "Guided Path":
		return RouteStrategyOrdered, nil
	case "Secret":
		return RouteStrategySecret, nil
	default:
		return 0, errors.New("invalid RouteStrategy")
	}
}

// ParseNavigationDisplayMode returns a NavigationDisplayMode from a string.
func ParseNavigationDisplayMode(s string) (NavigationDisplayMode, error) {
	switch s {
	case "Show Map", "Map Only":
		return NavigationDisplayMap, nil
	case "Show Map and Names", "Labelled Map":
		return NavigationDisplayMapAndNames, nil
	case "Show Location Names", "Location List":
		return NavigationDisplayNames, nil
	case "Show Clues", "Clue-Based":
		return NavigationDisplayClues, nil
	case "Custom Content", "Custom Clues":
		return NavigationDisplayCustom, nil
	case "Tasks":
		return NavigationDisplayTasks, nil
	default:
		return NavigationDisplayMap, errors.New("invalid NavigationDisplayMode")
	}
}

// ParseGameStatus returns a GameStatus from a string.
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
