package services

import (
	"context"
	"sort"

	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
)

// LeaderBoardService handles team ranking and leaderboard logic.
type LeaderBoardService struct {
	teamRepo repositories.TeamRepository
}

// NewLeaderBoardService creates a new LeaderBoardService.
func NewLeaderBoardService(teamRepo repositories.TeamRepository) *LeaderBoardService {
	return &LeaderBoardService{
		teamRepo: teamRepo,
	}
}

// GetLeaderBoardData returns sorted and ranked leaderboard data.
func (s *LeaderBoardService) GetLeaderBoardData(
	ctx context.Context,
	teams []models.Team,
	locationCount int,
	rankingScheme string,
	sortField string,
	sortOrder string,
) ([]LeaderBoardTeamData, error) {
	// Parse and validate parameters
	parsedRankingScheme := ParseRankingScheme(rankingScheme)
	parsedSortField := ParseSortField(sortField)
	parsedSortOrder := ParseSortOrder(sortOrder)
	// Convert teams to LeaderBoardTeamData
	leaderBoardData := make([]LeaderBoardTeamData, 0, len(teams))

	for _, team := range teams {
		if !team.HasStarted {
			continue
		}

		teamData := s.convertTeamToLeaderBoardData(team, locationCount)
		leaderBoardData = append(leaderBoardData, teamData)
	}

	// Apply ranking based on the specified scheme
	s.applyRanking(leaderBoardData, parsedRankingScheme)

	// Sort the data based on the specified field and order
	s.sortLeaderBoardData(leaderBoardData, parsedSortField, parsedSortOrder)

	return leaderBoardData, nil
}

// convertTeamToLeaderBoardData converts a models.Team to LeaderBoardTeamData.
func (s *LeaderBoardService) convertTeamToLeaderBoardData(team models.Team, locationCount int) LeaderBoardTeamData {
	checkInCount := len(team.CheckIns)

	// Find the most recent check-in time for accurate tiebreaker
	lastSeen := team.UpdatedAt
	for _, checkIn := range team.CheckIns {
		// Use TimeOut if available (completed check-in), otherwise TimeIn (current check-in)
		checkInTime := checkIn.TimeIn
		if !checkIn.TimeOut.IsZero() {
			checkInTime = checkIn.TimeOut
		}

		// Keep the most recent check-in time
		if checkInTime.After(lastSeen) {
			lastSeen = checkInTime
		}
	}

	return LeaderBoardTeamData{
		ID:           team.ID,
		Code:         team.Code,
		Name:         team.Name,
		Points:       team.Points,
		LastSeen:     lastSeen,
		Progress:     checkInCount,
		Status:       s.determineTeamStatus(team, locationCount),
		HasStarted:   team.HasStarted,
		MustCheckOut: team.MustCheckOut,
		CheckInCount: checkInCount,
	}
}

// determineTeamStatus determines the current status of a team.
func (s *LeaderBoardService) determineTeamStatus(team models.Team, locationCount int) TeamStatus {
	if team.MustCheckOut != "" {
		return StatusOnsite
	}

	checkInCount := len(team.CheckIns)
	if checkInCount > 0 {
		if locationCount > 0 && checkInCount == locationCount {
			return StatusFinished
		}
		return StatusTransit
	}

	return StatusStarted
}

// applyRanking applies ranking based on the specified scheme
// All ranking schemes use earliest last check-in time as tiebreaker to ensure no tied ranks.
func (s *LeaderBoardService) applyRanking(data []LeaderBoardTeamData, scheme RankingScheme) {
	switch scheme {
	case RankByProgress:
		s.rankByProgress(data)
	case RankByPoints:
		s.rankByPoints(data)
	case RankByCompletion:
		s.rankByCompletion(data)
	case RankByTimeToFirst:
		// TODO: Implement when we have check-in times
		s.rankByProgress(data) // Fallback for now
	case RankByTimeToLast:
		// TODO: Implement when we have check-in times
		s.rankByProgress(data) // Fallback for now
	default:
		s.rankByProgress(data)
	}
}

// rankByProgress ranks teams by their progress (number of check-ins).
func (s *LeaderBoardService) rankByProgress(data []LeaderBoardTeamData) {
	// Sort by progress descending, then by last seen ascending (earlier is better)
	sort.Slice(data, func(i, j int) bool {
		if data[i].Progress == data[j].Progress {
			return data[i].LastSeen.Before(data[j].LastSeen)
		}
		return data[i].Progress > data[j].Progress
	})

	// Assign sequential ranks - no ties allowed
	// Teams are already sorted by progress desc, then by LastSeen asc (earlier is better)
	for i := range data {
		data[i].Rank = i + 1
	}
}

// rankByPoints ranks teams by their points.
func (s *LeaderBoardService) rankByPoints(data []LeaderBoardTeamData) {
	// Sort by points descending, then by last seen ascending (earlier is better)
	sort.Slice(data, func(i, j int) bool {
		if data[i].Points == data[j].Points {
			return data[i].LastSeen.Before(data[j].LastSeen)
		}
		return data[i].Points > data[j].Points
	})

	// Assign sequential ranks - no ties allowed
	// Teams are already sorted by points desc, then by LastSeen asc (earlier is better)
	for i := range data {
		data[i].Rank = i + 1
	}
}

// rankByCompletion ranks teams by completion status, then by progress.
func (s *LeaderBoardService) rankByCompletion(data []LeaderBoardTeamData) {
	// Sort by completion status, then by progress, then by last seen ascending (earlier is better)
	sort.Slice(data, func(i, j int) bool {
		iFinished := data[i].Status == StatusFinished
		jFinished := data[j].Status == StatusFinished

		if iFinished != jFinished {
			return iFinished // Finished teams come first
		}

		if data[i].Progress == data[j].Progress {
			return data[i].LastSeen.Before(data[j].LastSeen)
		}
		return data[i].Progress > data[j].Progress
	})

	// Assign sequential ranks - no ties allowed
	// Teams are already sorted by completion status, then by progress desc, then by LastSeen asc
	for i := range data {
		data[i].Rank = i + 1
	}
}

// sortLeaderBoardData sorts the leaderboard data by the specified field and order.
func (s *LeaderBoardService) sortLeaderBoardData(data []LeaderBoardTeamData, field SortField, order SortOrder) {
	sort.Slice(data, func(i, j int) bool {
		var result bool

		switch field {
		case SortByRank:
			result = data[i].Rank < data[j].Rank
		case SortByCode:
			result = data[i].Code < data[j].Code
		case SortByName:
			result = data[i].Name < data[j].Name
		case SortByPoints:
			result = data[i].Points < data[j].Points
		case SortByLastSeen:
			result = data[i].LastSeen.Before(data[j].LastSeen)
		case SortByProgress:
			result = data[i].Progress < data[j].Progress
		case SortByStatus:
			result = string(data[i].Status) < string(data[j].Status)
		default:
			result = data[i].Rank < data[j].Rank
		}

		if order == SortDesc {
			return !result
		}
		return result
	})
}

// GetDefaultSortForRankingScheme returns the default sort field for a ranking scheme.
func (s *LeaderBoardService) GetDefaultSortForRankingScheme(scheme string) string {
	parsedScheme := ParseRankingScheme(scheme)
	switch parsedScheme {
	case RankByPoints:
		return string(SortByPoints)
	case RankByProgress:
		return string(SortByProgress)
	case RankByCompletion, RankByTimeToFirst, RankByTimeToLast:
		return string(SortByRank)
	default:
		return string(SortByRank)
	}
}

// GetSupportedRankingSchemes returns all supported ranking schemes.
func (s *LeaderBoardService) GetSupportedRankingSchemes() []string {
	return []string{
		string(RankByProgress),
		string(RankByPoints),
		string(RankByCompletion),
		string(RankByTimeToFirst),
		string(RankByTimeToLast),
	}
}

// GetSupportedSortFields returns all supported sort fields.
func (s *LeaderBoardService) GetSupportedSortFields() []string {
	return []string{
		string(SortByRank),
		string(SortByCode),
		string(SortByName),
		string(SortByPoints),
		string(SortByLastSeen),
		string(SortByProgress),
		string(SortByStatus),
	}
}

// GetSupportedSortOrders returns all supported sort orders.
func (s *LeaderBoardService) GetSupportedSortOrders() []string {
	return []string{
		string(SortAsc),
		string(SortDesc),
	}
}

// ParseSortField parses a string to SortField.
func ParseSortField(field string) SortField {
	switch field {
	case string(SortByRank):
		return SortByRank
	case string(SortByCode):
		return SortByCode
	case string(SortByName):
		return SortByName
	case string(SortByPoints):
		return SortByPoints
	case string(SortByLastSeen):
		return SortByLastSeen
	case string(SortByProgress):
		return SortByProgress
	case string(SortByStatus):
		return SortByStatus
	default:
		return SortByRank
	}
}

// ParseSortOrder parses a string to SortOrder.
func ParseSortOrder(order string) SortOrder {
	switch order {
	case string(SortDesc):
		return SortDesc
	case string(SortAsc):
		return SortAsc
	default:
		return SortAsc
	}
}

// ParseRankingScheme parses a string to RankingScheme.
func ParseRankingScheme(scheme string) RankingScheme {
	switch scheme {
	case string(RankByPoints):
		return RankByPoints
	case string(RankByProgress):
		return RankByProgress
	case string(RankByTimeToFirst):
		return RankByTimeToFirst
	case string(RankByTimeToLast):
		return RankByTimeToLast
	case string(RankByCompletion):
		return RankByCompletion
	default:
		return RankByProgress
	}
}
