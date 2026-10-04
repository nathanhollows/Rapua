package services

import (
	"time"
)

const (
	always = "always"
	day    = "day"
	week   = "week"
	month  = "month"
	year   = "year"
)

// BandUpdate carries the completion band as the form submitted it. An explicit
// 0 differs from an omitted bound (see game.FillBand), so both are pointers.
type BandUpdate struct {
	Min *int
	Max *int
}

// ObjectiveUpdateData holds the fields an update names. A nil field means
// "leave unchanged," which is what lets a control that touches one setting,
// like the tree's eye toggle, submit without clearing everything it does not
// name.
type ObjectiveUpdateData struct {
	Title string
	// Draft publishes or parks the objective. A pointer because the form may
	// not be offering the control at all, and "not mentioned" has to differ
	// from "publish this".
	Draft *bool
	// Routing names the route strategy. Nil leaves it alone; a set value is
	// validated before it is applied.
	Routing *string
	// MaxNext caps how many children a randomised node offers at once.
	MaxNext *int
	// Band carries the completion band. Nil is "the form did not offer it",
	// which is how the eye toggle leaves a section's band alone. A non-nil
	// Band with nil bounds is the author clearing it, which the UI offers as
	// "leave both blank to require all children" and a bare *int pair could
	// not express: nil meant both "unchanged" and "cleared".
	Band        *BandUpdate
	FinishLabel *string
	Color       *string
}

// LeaderBoardTeamData represents a team's data for leaderboard display.
type LeaderBoardTeamData struct {
	ID         string
	Code       string
	Name       string
	Points     int
	LastSeen   time.Time
	Progress   int
	Status     TeamStatus
	Rank       int
	HasStarted bool
}

// TeamStatus represents the current status of a team.
type TeamStatus string

const (
	StatusStarted  TeamStatus = "started"
	StatusTransit  TeamStatus = "transit"
	StatusFinished TeamStatus = "finished"
)

// SortField represents the field to sort by.
type SortField string

const (
	SortByRank     SortField = "rank"
	SortByCode     SortField = "code"
	SortByName     SortField = "name"
	SortByPoints   SortField = "points"
	SortByLastSeen SortField = "last_seen"
	SortByProgress SortField = "progress"
	SortByStatus   SortField = "status"
)

// SortOrder represents the sort direction.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// RankingScheme represents different ways to rank teams.
type RankingScheme string

const (
	RankByProgress   RankingScheme = "progress"
	RankByPoints     RankingScheme = "points"
	RankByCompletion RankingScheme = "completion"
)
