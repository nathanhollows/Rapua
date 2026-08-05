package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"math/rand/v2"

	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

type RunCreditService interface {
	DeductCreditForRunStartWithTx(ctx context.Context, tx *bun.Tx, userID, teamID, questID string) error
}

const (
	runCodeLength = 4
	batchSize     = 100
	// maxBatchRetries bounds how many times a batch is regenerated after a code
	// collision, so an exhausted code space fails loudly instead of hanging.
	maxBatchRetries = 10
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

type RunService struct {
	transactor     db.Transactor
	teamRepo       repositories.RunRepository
	checkInRepo    repositories.CheckInRepository
	creditService  RunCreditService
	blockStateRepo repositories.BlockStateRepository
	locationRepo   repositories.LocationRepository
	varStateRepo   repositories.RunVarStateRepository
	batchSize      int
}

// NewRunService creates a new RunService.
func NewRunService(
	transactor db.Transactor,
	tr repositories.RunRepository,
	ci repositories.CheckInRepository,
	creditService RunCreditService,
	bsr repositories.BlockStateRepository,
	lr repositories.LocationRepository,
	varStateRepo repositories.RunVarStateRepository,
) *RunService {
	return &RunService{
		transactor:     transactor,
		teamRepo:       tr,
		checkInRepo:    ci,
		creditService:  creditService,
		blockStateRepo: bsr,
		locationRepo:   lr,
		varStateRepo:   varStateRepo,
		batchSize:      batchSize,
	}
}

type RunActivity struct {
	Team      models.Run
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
func (s *RunService) containsCode(teams []models.Run, code string) bool {
	for _, team := range teams {
		if team.Code == code {
			return true
		}
	}
	return false
}

// generateRuns builds size runs for the quest, with codes unique within the batch.
func (s *RunService) generateRuns(questID string, size int) []models.Run {
	teams := make([]models.Run, 0, size)
	for range size {
		for {
			code := newCode(runCodeLength)
			if !s.containsCode(teams, code) {
				teams = append(teams, models.Run{Code: code, QuestID: questID})
				break
			}
		}
	}
	return teams
}

// AddTeams generates and inserts teams in batches, retrying with fresh codes if
// a batch collides with codes already in the database.
func (s *RunService) AddTeams(ctx context.Context, questID string, count int) ([]models.Run, error) {
	var newTeams []models.Run
	for i := 0; i < count; i += s.batchSize {
		size := min(s.batchSize, count-i)

		inserted := false
		for range maxBatchRetries {
			teams := s.generateRuns(questID, size)

			err := s.teamRepo.InsertBatch(ctx, teams)
			if err == nil {
				newTeams = append(newTeams, teams...)
				inserted = true
				break
			}
			// Any error other than a code collision is not worth retrying.
			if !errors.Is(err, repositories.ErrUniqueConstraint) {
				return nil, err
			}
		}

		if !inserted {
			return nil, fmt.Errorf(
				"generating %d unique run codes for quest %s: still colliding after %d attempts",
				size, questID, maxBatchRetries,
			)
		}
	}

	return newTeams, nil
}

// FindAll returns all teams for an instance.
func (s *RunService) FindAll(ctx context.Context, questID string) ([]models.Run, error) {
	return s.teamRepo.FindAll(ctx, questID)
}

// GetRunByCode returns a team by code.
func (s *RunService) GetRunByCode(ctx context.Context, code string) (*models.Run, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	return s.teamRepo.GetByCode(ctx, code)
}

// GetRunActivityOverview returns a list of teams and their activity.
func (s *RunService) GetRunActivityOverview(
	ctx context.Context,
	questID string,
	locations []models.Location,
) ([]RunActivity, error) {
	teams, err := s.teamRepo.FindAll(ctx, questID)
	if err != nil {
		return nil, err
	}

	var activity []RunActivity

	for _, team := range teams {
		if !team.HasStarted {
			continue
		}

		teamActivity := RunActivity{
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
func (s *RunService) Update(ctx context.Context, team *models.Run) error {
	return s.teamRepo.Update(ctx, team)
}

// AwardPoints awards points to a team.
func (s *RunService) AwardPoints(ctx context.Context, team *models.Run, points int) error {
	team.Points += points
	return s.teamRepo.Update(ctx, team)
}

// LoadQuest loads the run's quest, along with its settings and locations.
func (s *RunService) LoadQuest(ctx context.Context, team *models.Run) error {
	return s.teamRepo.LoadQuest(ctx, team)
}

// LoadCheckIns loads the run's check-ins, most recent first.
func (s *RunService) LoadCheckIns(ctx context.Context, team *models.Run) error {
	return s.teamRepo.LoadCheckIns(ctx, team)
}

// LoadBlockingLocation loads the location the run must check out of, if any.
func (s *RunService) LoadBlockingLocation(ctx context.Context, team *models.Run) error {
	return s.teamRepo.LoadBlockingLocation(ctx, team)
}

// LoadMessages loads the run's notifications, most recent first.
func (s *RunService) LoadMessages(ctx context.Context, team *models.Run) error {
	return s.teamRepo.LoadMessages(ctx, team)
}

// LoadRelations loads all relations for a team.
func (s *RunService) LoadRelations(ctx context.Context, team *models.Run) error {
	if err := s.teamRepo.LoadRelations(ctx, team); err != nil {
		return err
	}

	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return err
	}
	team.VarStates = varStates

	return nil
}

func (s *RunService) StartPlaying(ctx context.Context, runCode string) error {
	runCode = strings.TrimSpace(strings.ToUpper(runCode))

	team, err := s.GetRunByCode(ctx, runCode)
	if err != nil {
		return ErrTeamNotFound
	}

	if team.HasStarted {
		return nil
	}

	userID, err := s.teamRepo.GetUserIDByCode(ctx, runCode)
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

	err = s.creditService.DeductCreditForRunStartWithTx(ctx, tx, userID, team.ID, team.QuestID)
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
func (s *RunService) BuildLocationGroupMap(structure *models.GameStructure) map[string]LocationGroupInfo {
	result := make(map[string]LocationGroupInfo)
	s.buildLocationGroupMapRecursive(structure, result)
	return result
}

func (s *RunService) buildLocationGroupMapRecursive(group *models.GameStructure, result map[string]LocationGroupInfo) {
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
func (s *RunService) BuildGroupOrder(structure *models.GameStructure) map[string]int {
	result := make(map[string]int)
	order := 0
	s.buildGroupOrderRecursive(structure, result, &order)
	return result
}

func (s *RunService) buildGroupOrderRecursive(group *models.GameStructure, result map[string]int, order *int) {
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

// insertCheckInSorted inserts a check-in into the group's check-ins in sorted order by creation time.
func insertCheckInSorted(group *GroupedCheckIns, checkIn models.CheckIn) {
	insertPos := sort.Search(len(group.CheckIns), func(i int) bool {
		return group.CheckIns[i].CreatedAt.Before(checkIn.CreatedAt)
	})
	// Efficient insertion: append and copy instead of double append
	group.CheckIns = append(group.CheckIns, models.CheckIn{})
	copy(group.CheckIns[insertPos+1:], group.CheckIns[insertPos:])
	group.CheckIns[insertPos] = checkIn
}

// groupWithOrder tracks a group name and its order in the game structure.
type groupWithOrder struct {
	name  string
	order int
	found bool
}

// sortGroupsByOrder sorts groups by their order in the game structure.
// Groups with defined order come before groups without order.
func sortGroupsByOrder(groupMap map[string]*GroupedCheckIns, groupOrder map[string]int) []GroupedCheckIns {
	groups := make([]groupWithOrder, 0, len(groupMap))
	for name := range groupMap {
		order, found := groupOrder[name]
		groups = append(groups, groupWithOrder{name: name, order: order, found: found})
	}

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

	result := make([]GroupedCheckIns, 0, len(groupMap))
	for _, g := range groups {
		result = append(result, *groupMap[g.name])
	}
	return result
}

// GroupCheckInsByGroup groups check-ins by their location's group and sorts by game structure order.
// Optimized to minimize passes over the data by sorting during grouping.
func (s *RunService) GroupCheckInsByGroup(
	checkIns []models.CheckIn,
	locationGroups map[string]LocationGroupInfo,
	groupOrder map[string]int,
) []GroupedCheckIns {
	groupMap := make(map[string]*GroupedCheckIns)
	var ungrouped []models.CheckIn

	// Single pass: group check-ins and insert in sorted order by creation time
	for _, scan := range checkIns {
		if scan.MustCheckOut {
			continue
		}

		groupInfo, ok := locationGroups[scan.Location.ID]
		if !ok {
			ungrouped = append(ungrouped, scan)
			continue
		}

		if _, exists := groupMap[groupInfo.GroupName]; !exists {
			groupMap[groupInfo.GroupName] = &GroupedCheckIns{
				GroupInfo: groupInfo,
				CheckIns:  []models.CheckIn{},
			}
		}
		insertCheckInSorted(groupMap[groupInfo.GroupName], scan)
	}

	// Sort ungrouped check-ins by creation time (reverse chronological)
	sort.Slice(ungrouped, func(i, j int) bool {
		return ungrouped[i].CreatedAt.After(ungrouped[j].CreatedAt)
	})

	// Build result slice in sorted order by group order
	result := sortGroupsByOrder(groupMap, groupOrder)

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

// newCode generates an alpha string of easily recognisable characters.
// Confusing letters such as I and L, O and Q have one pair removed.
func newCode(length int) string {
	symbols := []rune("ABCDEFGHJKLMNPRSTUVWXYZ")
	b := make([]rune, length)
	for i := range length {
		b[i] = symbols[rand.IntN(len(symbols))] //nolint:gosec // Team codes do not need cryptographic randomness
	}
	return string(b)
}
