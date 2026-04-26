package game

import "errors"

// RouteStrategy defines how players navigate through locations.
type RouteStrategy string

const (
	RouteStrategyRandomised RouteStrategy = "randomised"
	RouteStrategyFreeRoam   RouteStrategy = "free_roam"
	RouteStrategyOrdered    RouteStrategy = "ordered"
	RouteStrategySecret     RouteStrategy = "secret" // Locations that may be accessed out of sequence
)

// CompletionType defines how a group is considered completed.
type CompletionType string

const (
	CompletionAll     CompletionType = "all"
	CompletionMinimum CompletionType = "minimum"
)

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

// ParseRouteStrategy returns a RouteStrategy from its canonical string value.
func ParseRouteStrategy(s string) (RouteStrategy, error) {
	switch s {
	case "randomised":
		return RouteStrategyRandomised, nil
	case "free_roam":
		return RouteStrategyFreeRoam, nil
	case "ordered":
		return RouteStrategyOrdered, nil
	case "secret":
		return RouteStrategySecret, nil
	default:
		return "", errors.New("invalid RouteStrategy")
	}
}

// String returns the display name of the CompletionType.
func (c CompletionType) String() string {
	switch c {
	case CompletionAll:
		return "All Locations"
	case CompletionMinimum:
		return "Minimum Required"
	default:
		return string(c)
	}
}

// Description returns the description of the CompletionType.
func (c CompletionType) Description() string {
	switch c {
	case CompletionAll:
		return "All locations must be completed for the group to be considered done."
	case CompletionMinimum:
		return "A minimum number of locations must be completed for the group to be considered done."
	default:
		return ""
	}
}
