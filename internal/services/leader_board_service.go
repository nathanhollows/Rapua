package services

import (
	"context"
	"sort"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
)

// LeaderBoardService handles team ranking and leaderboard logic.
type LeaderBoardService struct{}

// NewLeaderBoardService creates a new LeaderBoardService.
func NewLeaderBoardService() *LeaderBoardService {
	return &LeaderBoardService{}
}

// GetLeaderBoardData returns sorted and ranked data. lastSeen and summaries are keyed by run code.
func (s *LeaderBoardService) GetLeaderBoardData(
	_ context.Context,
	teams []models.Run,
	lastSeen map[string]time.Time,
	summaries map[string]RunSummary,
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

		teamData := s.convertTeamToLeaderBoardData(team, lastSeen[team.Code], summaries[team.Code])
		leaderBoardData = append(leaderBoardData, teamData)
	}

	// Apply ranking based on the specified scheme
	s.applyRanking(leaderBoardData, parsedRankingScheme)

	// Sort the data based on the specified field and order
	s.sortLeaderBoardData(leaderBoardData, parsedSortField, parsedSortOrder)

	return leaderBoardData, nil
}

// convertTeamToLeaderBoardData converts a models.Run to LeaderBoardTeamData.
// A run that has touched no block falls back to when it started, which is the
// last thing known about it. A zero value there would read as older than every
// other run and quietly win the ranking tiebreak.
func (s *LeaderBoardService) convertTeamToLeaderBoardData(
	team models.Run, lastSeen time.Time, summary RunSummary,
) LeaderBoardTeamData {
	if lastSeen.IsZero() {
		lastSeen = team.StartedAt
	}
	if summary.Status == "" {
		// A started run with an empty stream has no summary.
		summary.Status = StatusStarted
	}

	return LeaderBoardTeamData{
		ID:         team.ID,
		Code:       team.Code,
		Name:       team.Name,
		Points:     team.Points,
		LastSeen:   lastSeen,
		Progress:   summary.Done,
		Total:      summary.Total,
		Status:     summary.Status,
		HasStarted: team.HasStarted,
	}
}

// fractionBefore compares shares, not counts: a run that finished a cut-short
// branch would otherwise rank below one halfway through a long one, against its bar.
func fractionBefore(a, b LeaderBoardTeamData) bool {
	if a.Total > 0 && b.Total > 0 {
		return a.Progress*b.Total > b.Progress*a.Total
	}
	return a.Progress > b.Progress
}

// fractionTied is defined through fractionBefore so the tie and order tests
// cannot disagree, which would leave sort.Slice's result undefined.
func fractionTied(a, b LeaderBoardTeamData) bool {
	return !fractionBefore(a, b) && !fractionBefore(b, a)
}

// All ranking schemes use earliest last-updated time as tiebreaker to ensure no tied ranks.
func (s *LeaderBoardService) applyRanking(data []LeaderBoardTeamData, scheme RankingScheme) {
	switch scheme {
	case RankByProgress:
		s.rankByProgress(data)
	case RankByPoints:
		s.rankByPoints(data)
	case RankByCompletion:
		s.rankByCompletion(data)
	default:
		s.rankByProgress(data)
	}
}

func (s *LeaderBoardService) rankByProgress(data []LeaderBoardTeamData) {
	// Earlier last-seen wins a tie.
	sort.Slice(data, func(i, j int) bool {
		if fractionTied(data[i], data[j]) {
			return data[i].LastSeen.Before(data[j].LastSeen)
		}
		return fractionBefore(data[i], data[j])
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

		if fractionTied(data[i], data[j]) {
			return data[i].LastSeen.Before(data[j].LastSeen)
		}
		return fractionBefore(data[i], data[j])
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
			result = fractionBefore(data[j], data[i])
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
	case RankByCompletion:
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
	case string(RankByCompletion):
		return RankByCompletion
	default:
		return RankByProgress
	}
}
