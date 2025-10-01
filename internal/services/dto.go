package services

import "time"

// LocationData is the data required to update a new location. Blank
// fields are ignored, with the exception of Clues and ClueIDs which
// are always required.
type LocationUpdateData struct {
	Name      string
	Latitude  float64
	Longitude float64
	Points    int
}

// LeaderBoardTeamData represents a team's data for leaderboard display.
type LeaderBoardTeamData struct {
	ID           string
	Code         string
	Name         string
	Points       int
	LastSeen     time.Time
	Progress     int
	Status       TeamStatus
	Rank         int
	HasStarted   bool
	MustCheckOut string
	CheckInCount int
}

// TeamStatus represents the current status of a team.
type TeamStatus string

const (
	StatusStarted  TeamStatus = "started"
	StatusOnsite   TeamStatus = "onsite"
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
	RankByProgress    RankingScheme = "progress"
	RankByPoints      RankingScheme = "points"
	RankByTimeToFirst RankingScheme = "time_to_first"
	RankByTimeToLast  RankingScheme = "time_to_last"
	RankByCompletion  RankingScheme = "completion"
)
