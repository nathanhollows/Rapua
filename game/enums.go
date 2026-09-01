package game

import "errors"

// RouteStrategy defines how players navigate through objectives.
type RouteStrategy string

const (
	RouteStrategyRandomised RouteStrategy = "randomised"
	RouteStrategyFreeRoam   RouteStrategy = "free_roam"
	RouteStrategyOrdered    RouteStrategy = "ordered"
	// RouteStrategySecret is retired: lint rejects a document carrying it, and
	// an objective is reached instead through its parent's routing, its depends
	// and a scan block in its proof. It survives only because the group blob
	// and the code reading that blob still reference it.
	RouteStrategySecret RouteStrategy = "secret"
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
		return "The game will randomly select objectives for players to pursue. Good for large groups as it disperses players."
	case RouteStrategyFreeRoam:
		return "Players can pursue objectives in any order. This mode shows all objectives and is good for exploration."
	case RouteStrategyOrdered:
		return "Players must complete objectives in a specific order. Good for narrative experiences."
	case RouteStrategySecret:
		return "Objectives that may be accessed out of sequence. These objectives are never explicitly shown to players."
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
		return "All Objectives"
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
		return "All objectives must be completed for the group to be considered done."
	case CompletionMinimum:
		return "A minimum number of objectives must be completed for the group to be considered done."
	default:
		return ""
	}
}
