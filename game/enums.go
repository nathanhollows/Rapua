package game

import "errors"

// RouteStrategy defines how players navigate through objectives.
type RouteStrategy string

const (
	RouteStrategyRandomised RouteStrategy = "randomised"
	RouteStrategyFreeRoam   RouteStrategy = "free_roam"
	RouteStrategyOrdered    RouteStrategy = "ordered"
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

// ProgressFeedback is what a quest tells a player about how far through they
// are. Each value is a different condition, so games compared across studies
// must say which a player had.
type ProgressFeedback string

const (
	// ProgressFeedbackFraction is completions out of the fewest the run needs.
	ProgressFeedbackFraction ProgressFeedback = "fraction"
	// ProgressFeedbackCount is completions alone, with no target.
	ProgressFeedbackCount ProgressFeedback = "count"
	ProgressFeedbackNone  ProgressFeedback = "none"
)

func ParseProgressFeedback(s string) (ProgressFeedback, error) {
	switch ProgressFeedback(s) {
	case ProgressFeedbackFraction, ProgressFeedbackCount, ProgressFeedbackNone:
		return ProgressFeedback(s), nil
	default:
		return "", errors.New("invalid ProgressFeedback")
	}
}

// Normalised reads unset as fraction, what every quest showed before the choice
// existed. An unknown value reads the same, so a bad row never means no feedback.
func (m ProgressFeedback) Normalised() ProgressFeedback {
	if parsed, err := ParseProgressFeedback(string(m)); err == nil {
		return parsed
	}
	return ProgressFeedbackFraction
}
