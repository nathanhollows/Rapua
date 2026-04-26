package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v7/game"
)

//nolint:recvcheck // Value() requires value receiver, Scan() requires pointer receiver per database/sql interface
type StrArray []string

// RouteStrategy is a type alias; re-exported from game/ so existing callers don't need to update imports.
type RouteStrategy = game.RouteStrategy

type GameStatus int
type Provider string

type RouteStrategies []RouteStrategy
type GameStatuses []GameStatus

const (
	RouteStrategyRandomised = game.RouteStrategyRandomised
	RouteStrategyFreeRoam   = game.RouteStrategyFreeRoam
	RouteStrategyOrdered    = game.RouteStrategyOrdered
	RouteStrategySecret     = game.RouteStrategySecret
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

// GetGameStatuses returns a list of game statuses.
func GetGameStatuses() GameStatuses {
	return []GameStatus{Scheduled, Active, Closed}
}

// String returns the string representation of the GameStatus.
func (g GameStatus) String() string {
	return [...]string{"Scheduled", "Active", "Closed"}[g]
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
	return game.ParseRouteStrategy(s)
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
