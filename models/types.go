package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

//nolint:recvcheck // Value() requires value receiver, Scan() requires pointer receiver per database/sql interface
type StrArray []string

type RouteStrategy string
type NavigationMode string
type GameStatus int
type Provider string

type RouteStrategies []RouteStrategy
type NavigationModes []NavigationMode
type GameStatuses []GameStatus

const (
	RouteStrategyRandomised RouteStrategy = "randomised"
	RouteStrategyFreeRoam   RouteStrategy = "free_roam"
	RouteStrategyOrdered    RouteStrategy = "ordered"
	RouteStrategySecret     RouteStrategy = "secret" // Locations that may be accessed out of sequence
)

const (
	NavigationMap         NavigationMode = "map"
	NavigationLabelledMap NavigationMode = "labelled_map"
	NavigationList        NavigationMode = "location_list"
	NavigationCustom      NavigationMode = "custom" // For Block content
	NavigationTasks       NavigationMode = "tasks"  // Task checklist with completion tracking
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

// GetRouteStrategies returns a list of route strategies.
func GetRouteStrategies() RouteStrategies {
	return []RouteStrategy{RouteStrategyOrdered, RouteStrategyFreeRoam, RouteStrategyRandomised, RouteStrategySecret}
}

// GetNavigationModes returns a list of navigation modes.
func GetNavigationModes() NavigationModes {
	return []NavigationMode{
		NavigationMap,
		NavigationLabelledMap,
		NavigationList,
		NavigationCustom,
		NavigationTasks,
	}
}

// GetGameStatuses returns a list of game statuses.
func GetGameStatuses() GameStatuses {
	return []GameStatus{Scheduled, Active, Closed}
}

// String returns the display name of the RouteStrategy.
func (n RouteStrategy) String() string {
	switch n {
	case RouteStrategyRandomised:
		return "Randomised Route"
	case RouteStrategyFreeRoam:
		return "Open Exploration"
	case RouteStrategyOrdered:
		return "Guided Path"
	case RouteStrategySecret:
		return "Secret"
	default:
		return string(n)
	}
}

// String returns the display name of the NavigationMode.
func (n NavigationMode) String() string {
	switch n {
	case NavigationMap:
		return "Map Only"
	case NavigationLabelledMap:
		return "Labelled Map"
	case NavigationList:
		return "Location List"
	case NavigationCustom:
		return "Custom Clues"
	case NavigationTasks:
		return "Tasks"
	default:
		return string(n)
	}
}

// String returns the string representation of the GameStatus.
func (g GameStatus) String() string {
	return [...]string{"Scheduled", "Active", "Closed"}[g]
}

// Description returns the description of the RouteStrategy.
func (n RouteStrategy) Description() string {
	switch n {
	case RouteStrategyRandomised:
		return "The game will randomly select locations for players to visit. Good for large groups as it disperses players."
	case RouteStrategyFreeRoam:
		return "Players can visit locations in any order. This mode shows all locations and is good for exploration."
	case RouteStrategyOrdered:
		return "Players must visit locations in a specific order. Good for narrative experiences."
	case RouteStrategySecret:
		return "Locations that may be accessed out of sequence. These locations are never explicitly shown to players."
	default:
		return ""
	}
}

// Description returns the description of the NavigationMode.
func (n NavigationMode) Description() string {
	switch n {
	case NavigationMap:
		return "Players are shown a map."
	case NavigationLabelledMap:
		return "Players are shown a map with location names."
	case NavigationList:
		return "Players are shown a list of locations by name."
	case NavigationCustom:
		return "Players are shown custom content, e.g., randomised clues or images, using the block builder."
	case NavigationTasks:
		return "Players see a checklist, like a scavenger hunt, with completion tracking."
	default:
		return ""
	}
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
	case "randomised", "Random", "Randomised Route":
		return RouteStrategyRandomised, nil
	case "free_roam", "Free Roam", "Open Exploration":
		return RouteStrategyFreeRoam, nil
	case "ordered", "Ordered", "Guided Path":
		return RouteStrategyOrdered, nil
	case "secret", "Secret":
		return RouteStrategySecret, nil
	default:
		return "", errors.New("invalid RouteStrategy")
	}
}

// ParseNavigationMode returns a NavigationMode from a string.
func ParseNavigationMode(s string) (NavigationMode, error) {
	switch s {
	case "map", "Show Map", "Map Only":
		return NavigationMap, nil
	case "labelled_map", "Show Map and Names", "Labelled Map":
		return NavigationLabelledMap, nil
	case "location_list", "Show Location Names", "Location List":
		return NavigationList, nil
	case "custom", "Custom Content", "Custom Clues":
		return NavigationCustom, nil
	case "tasks", "Tasks":
		return NavigationTasks, nil
	default:
		return "", errors.New("invalid NavigationMode")
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
