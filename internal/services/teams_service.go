package services

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/helpers"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/uptrace/bun"
)

type TeamCreditService interface {
	DeductCreditForTeamStartWithTx(ctx context.Context, tx *bun.Tx, userID, teamID, instanceID string) error
}

const (
	teamCodeLength = 4
	batchSize      = 100
)

// LocationGroupInfo holds group information for a location.
type LocationGroupInfo struct {
	GroupName  string
	GroupColor string
}

// GroupedCheckIns represents check-ins grouped by location group.
type GroupedCheckIns struct {
	GroupInfo LocationGroupInfo
	CheckIns  []models.CheckIn
}

type TeamService struct {
	transactor     db.Transactor
	teamRepo       repositories.TeamRepository
	checkInRepo    repositories.CheckInRepository
	creditService  TeamCreditService
	blockStateRepo repositories.BlockStateRepository
	locationRepo   repositories.LocationRepository
	batchSize      int
}

// NewTeamService creates a new TeamService.
func NewTeamService(
	transactor db.Transactor,
	tr repositories.TeamRepository,
	ci repositories.CheckInRepository,
	creditService TeamCreditService,
	bsr repositories.BlockStateRepository,
	lr repositories.LocationRepository,
) *TeamService {
	return &TeamService{
		transactor:     transactor,
		teamRepo:       tr,
		checkInRepo:    ci,
		creditService:  creditService,
		blockStateRepo: bsr,
		locationRepo:   lr,
		batchSize:      batchSize,
	}
}

type TeamActivity struct {
	Team      models.Team
	Locations []LocationActivity
}

type LocationActivity struct {
	Location models.Location
	Visited  bool
	Visiting bool
	Duration float64
	TimeIn   time.Time
	TimeOut  time.Time
}

// Helper function to check for code uniqueness within a batch.
func (s *TeamService) containsCode(teams []models.Team, code string) bool {
	for _, team := range teams {
		if team.Code == code {
			return true
		}
	}
	return false
}

// AddTeams generates and inserts teams in batches, retrying if unique constraint errors occur.
func (s *TeamService) AddTeams(ctx context.Context, instanceID string, count int) ([]models.Team, error) {
	var newTeams []models.Team
	for i := 0; i < count; i += s.batchSize {
		size := min(s.batchSize, count-i)
		teams := make([]models.Team, 0, size)

		for range size {
			var team models.Team
			for {
				// TODO: Remove magic number
				code := helpers.NewCode(teamCodeLength)
				team = models.Team{
					Code:       code,
					InstanceID: instanceID,
				}

				// Ensure code uniqueness within the current batch
				if !s.containsCode(teams, code) {
					teams = append(teams, team)
					break
				}
			}
		}

		// Insert the batch and retry if there's a unique constraint error
		err := s.teamRepo.InsertBatch(ctx, teams)
		if err != nil {
			if errors.Is(err, errors.New("unique constraint error")) {
				i -= s.batchSize // Retry this batch
				continue
			}
			return nil, err
		}
		newTeams = append(newTeams, teams...)
	}

	return newTeams, nil
}

// FindAll returns all teams for an instance.
func (s *TeamService) FindAll(ctx context.Context, instanceID string) ([]models.Team, error) {
	return s.teamRepo.FindAll(ctx, instanceID)
}

// GetTeamByCode returns a team by code.
func (s *TeamService) GetTeamByCode(ctx context.Context, code string) (*models.Team, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	return s.teamRepo.GetByCode(ctx, code)
}

// GetTeamActivityOverview returns a list of teams and their activity.
func (s *TeamService) GetTeamActivityOverview(
	ctx context.Context,
	instanceID string,
	locations []models.Location,
) ([]TeamActivity, error) {
	teams, err := s.teamRepo.FindAll(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	var activity []TeamActivity

	for _, team := range teams {
		if !team.HasStarted {
			continue
		}

		teamActivity := TeamActivity{
			Team:      team,
			Locations: make([]LocationActivity, len(locations)),
		}

		for i, location := range locations {
			locationActivity := LocationActivity{
				Location: location,
				Visited:  false,
				Visiting: false,
				Duration: 0,
				TimeIn:   time.Time{},
				TimeOut:  time.Time{},
			}

			// Check if the team has visited the location
			for _, checkin := range team.CheckIns {
				if checkin.LocationID == location.Marker.Code {
					locationActivity.Visited = true
					locationActivity.TimeIn = checkin.TimeIn
					if checkin.TimeOut.IsZero() {
						locationActivity.Visiting = true
					} else {
						locationActivity.TimeOut = checkin.TimeOut
						locationActivity.Duration = checkin.TimeOut.Sub(checkin.TimeIn).Seconds()
					}
					break
				}
			}

			teamActivity.Locations[i] = locationActivity
		}

		activity = append(activity, teamActivity)
	}

	return activity, nil
}

// Update updates a team in the database.
func (s *TeamService) Update(ctx context.Context, team *models.Team) error {
	return s.teamRepo.Update(ctx, team)
}

// AwardPoints awards points to a team.
func (s *TeamService) AwardPoints(ctx context.Context, team *models.Team, points int) error {
	team.Points += points
	return s.teamRepo.Update(ctx, team)
}

// LoadRelation loads the specified relation for a team.
// Relations can be "Instance", "Scans", "BlockingLocation", or "Messages".
func (s *TeamService) LoadRelation(ctx context.Context, team *models.Team, relation string) error {
	switch relation {
	case "Instance":
		return s.teamRepo.LoadInstance(ctx, team)
	case "Scans":
		return s.teamRepo.LoadCheckIns(ctx, team)
	case "BlockingLocation":
		return s.teamRepo.LoadBlockingLocation(ctx, team)
	case "Messages":
		return s.teamRepo.LoadMessages(ctx, team)
	default:
		return errors.New("unknown relation")
	}
}

// LoadRelations loads all relations for a team.
func (s *TeamService) LoadRelations(ctx context.Context, team *models.Team) error {
	err := s.teamRepo.LoadRelations(ctx, team)
	if err != nil {
		return err
	}

	return nil
}

func (s *TeamService) StartPlaying(ctx context.Context, teamCode string) error {
	teamCode = strings.TrimSpace(strings.ToUpper(teamCode))

	team, err := s.GetTeamByCode(ctx, teamCode)
	if err != nil {
		return ErrTeamNotFound
	}

	if team.HasStarted {
		return nil
	}

	userID, err := s.teamRepo.GetUserIDByCode(ctx, teamCode)
	if err != nil {
		return errors.New("getting user ID for team: " + err.Error())
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errors.New("beginning transaction: " + err.Error())
	}
	// Ensure rollback on failure
	defer func() {
		if p := recover(); p != nil {
			err = tx.Rollback()
			if err != nil {
				panic("rolling back transaction after panic: " + err.Error())
			}
			panic(p)
		}
	}()

	err = s.creditService.DeductCreditForTeamStartWithTx(ctx, tx, userID, team.ID, team.InstanceID)
	if err != nil {
		txErr := tx.Rollback()
		if txErr != nil {
			panic("rolling back transaction after credit deduction failure: " + txErr.Error())
		}
		return errors.New("deducting credit for team start: " + err.Error())
	}

	err = s.teamRepo.UpdateTeamStartedWithTx(ctx, tx, team.Code)
	if err != nil {
		txErr := tx.Rollback()
		if txErr != nil {
			return errors.New("rolling back transaction after team start update failure: " + txErr.Error())
		}
		return errors.New("updating team as started: " + err.Error())
	}

	return tx.Commit()
}

// BuildLocationGroupMap creates a map from location ID to group info.
func (s *TeamService) BuildLocationGroupMap(structure *models.GameStructure) map[string]LocationGroupInfo {
	result := make(map[string]LocationGroupInfo)
	s.buildLocationGroupMapRecursive(structure, result)
	return result
}

func (s *TeamService) buildLocationGroupMapRecursive(group *models.GameStructure, result map[string]LocationGroupInfo) {
	// Skip root group (has no name/color)
	if !group.IsRoot {
		info := LocationGroupInfo{
			GroupName:  group.Name,
			GroupColor: group.Color,
		}
		// Map all locations in this group
		for _, locationID := range group.LocationIDs {
			result[locationID] = info
		}
	}
	// Recurse into subgroups
	for i := range group.SubGroups {
		s.buildLocationGroupMapRecursive(&group.SubGroups[i], result)
	}
}

// BuildGroupOrder creates a map from group name to its order in the game structure.
func (s *TeamService) BuildGroupOrder(structure *models.GameStructure) map[string]int {
	result := make(map[string]int)
	order := 0
	s.buildGroupOrderRecursive(structure, result, &order)
	return result
}

func (s *TeamService) buildGroupOrderRecursive(group *models.GameStructure, result map[string]int, order *int) {
	// Skip root group (has no name)
	if !group.IsRoot {
		result[group.Name] = *order
		*order++
	}
	// Recurse into subgroups
	for i := range group.SubGroups {
		s.buildGroupOrderRecursive(&group.SubGroups[i], result, order)
	}
}

// GroupCheckInsByGroup groups check-ins by their location's group and sorts by game structure order.
// Optimized to minimize passes over the data by sorting during grouping.
func (s *TeamService) GroupCheckInsByGroup(
	checkIns []models.CheckIn,
	locationGroups map[string]LocationGroupInfo,
	groupOrder map[string]int,
) []GroupedCheckIns {
	groupMap := make(map[string]*GroupedCheckIns)
	var ungrouped []models.CheckIn

	// Single pass: group check-ins and insert in sorted order by creation time
	for _, scan := range checkIns {
		if !scan.MustCheckOut {
			if groupInfo, ok := locationGroups[scan.Location.ID]; ok {
				if _, exists := groupMap[groupInfo.GroupName]; !exists {
					groupMap[groupInfo.GroupName] = &GroupedCheckIns{
						GroupInfo: groupInfo,
						CheckIns:  []models.CheckIn{},
					}
				}
				// Insert check-in in sorted position by creation time
				group := groupMap[groupInfo.GroupName]
				insertPos := sort.Search(len(group.CheckIns), func(i int) bool {
					return group.CheckIns[i].CreatedAt.After(scan.CreatedAt)
				})
				// Efficient insertion: append and copy instead of double append
				group.CheckIns = append(group.CheckIns, models.CheckIn{})
				copy(group.CheckIns[insertPos+1:], group.CheckIns[insertPos:])
				group.CheckIns[insertPos] = scan
			} else {
				ungrouped = append(ungrouped, scan)
			}
		}
	}

	// Sort ungrouped check-ins by creation time
	sort.Slice(ungrouped, func(i, j int) bool {
		return ungrouped[i].CreatedAt.Before(ungrouped[j].CreatedAt)
	})

	// Build result slice in sorted order by group order
	// First, collect all group names and their orders
	type groupWithOrder struct {
		name  string
		order int
		found bool
	}
	groups := make([]groupWithOrder, 0, len(groupMap))
	for name := range groupMap {
		order, found := groupOrder[name]
		groups = append(groups, groupWithOrder{name: name, order: order, found: found})
	}

	// Sort groups by their order
	sort.Slice(groups, func(i, j int) bool {
		// Groups with order come before those without
		if groups[i].found && !groups[j].found {
			return true
		}
		if !groups[i].found && groups[j].found {
			return false
		}
		// If both have order, sort by order value
		if groups[i].found && groups[j].found {
			return groups[i].order < groups[j].order
		}
		// If neither has order, maintain stable order
		return false
	})

	// Build final result in sorted order
	result := make([]GroupedCheckIns, 0, len(groupMap)+1)
	for _, g := range groups {
		result = append(result, *groupMap[g.name])
	}

	// Add ungrouped locations as "Other" group at the end
	if len(ungrouped) > 0 {
		result = append(result, GroupedCheckIns{
			GroupInfo: LocationGroupInfo{
				GroupName:  "Other",
				GroupColor: "base-content",
			},
			CheckIns: ungrouped,
		})
	}

	return result
}
