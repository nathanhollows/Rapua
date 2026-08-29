package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"math/rand/v2"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
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

type ObjectiveGroupInfo struct {
	GroupName  string
	GroupColor string
}

type RunService struct {
	transactor                     db.Transactor
	teamRepo                       repositories.RunRepository
	creditService                  RunCreditService
	blockStateRepo                 repositories.BlockStateRepository
	varStateRepo                   repositories.RunVarStateRepository
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
	batchSize                      int
}

func NewRunService(
	transactor db.Transactor,
	tr repositories.RunRepository,
	creditService RunCreditService,
	bsr repositories.BlockStateRepository,
	varStateRepo repositories.RunVarStateRepository,
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
) *RunService {
	return &RunService{
		transactor:                     transactor,
		teamRepo:                       tr,
		creditService:                  creditService,
		blockStateRepo:                 bsr,
		varStateRepo:                   varStateRepo,
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
		batchSize:                      batchSize,
	}
}

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

// GetRunByCode returns a run by code.
func (s *RunService) GetRunByCode(ctx context.Context, code string) (*models.Run, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	return s.teamRepo.GetByCode(ctx, code)
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

// LoadQuest loads the run's quest, along with its settings.
func (s *RunService) LoadQuest(ctx context.Context, team *models.Run) error {
	return s.teamRepo.LoadQuest(ctx, team)
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

func (s *RunService) BuildObjectiveGroupMap(structure *models.GameStructure) map[string]ObjectiveGroupInfo {
	result := make(map[string]ObjectiveGroupInfo)
	s.buildObjectiveGroupMapRecursive(structure, result)
	return result
}

func (s *RunService) buildObjectiveGroupMapRecursive(group *models.GameStructure, result map[string]ObjectiveGroupInfo) {
	// Skip root group (has no name/color).
	if !group.IsRoot {
		info := ObjectiveGroupInfo{
			GroupName:  group.Name,
			GroupColor: group.Color,
		}
		for _, objectiveID := range group.ObjectiveIDs {
			result[objectiveID] = info
		}
	}
	for i := range group.SubGroups {
		s.buildObjectiveGroupMapRecursive(&group.SubGroups[i], result)
	}
}

// GetIncompleteObjectives returns the quest's reveal-context objectives still
// incomplete for this run: an unordered "what's outstanding" list for admin
// dashboards.
func (s *RunService) GetIncompleteObjectives(ctx context.Context, questID, runCode string) ([]models.Objective, error) {
	objectives, _, completed, err := s.objectivesWithCompletion(ctx, questID, runCode)
	if err != nil {
		return nil, err
	}

	incomplete := make([]models.Objective, 0, len(objectives))
	for _, objective := range objectives {
		if !completed[objective.ID] {
			incomplete = append(incomplete, objective)
		}
	}
	return incomplete, nil
}

// objectivesWithCompletion fetches every objective for a quest, alongside which
// ones are complete for a run (both as a completion-order ID slice and a lookup
// set): the shared core of GetIncompleteObjectives and GetCompletedObjectives,
// so the definition of "complete" (reveal-context, from the append-only
// completion log) lives in one place instead of drifting between them.
func (s *RunService) objectivesWithCompletion(
	ctx context.Context, questID, runCode string,
) (objectives []models.Objective, completedIDsOrdered []string, completed map[string]bool, err error) {
	objectives, err = s.objectiveRepo.FindByQuestID(ctx, questID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("finding objectives: %w", err)
	}

	completedIDsOrdered, err = s.objectiveContextCompletionRepo.FindCompletedObjectiveIDsOrdered(
		ctx, runCode, game.ContextObjectiveReveal,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("finding completed objectives: %w", err)
	}

	completed = make(map[string]bool, len(completedIDsOrdered))
	for _, id := range completedIDsOrdered {
		completed[id] = true
	}
	return objectives, completedIDsOrdered, completed, nil
}

// GetCompletedObjectives is GetIncompleteObjectives' complement: the quest's
// reveal-context objectives already completed for this run, for the player's
// /journal, in completion order (most recent first).
func (s *RunService) GetCompletedObjectives(ctx context.Context, questID, runCode string) ([]models.Objective, error) {
	objectives, completedIDsOrdered, _, err := s.objectivesWithCompletion(ctx, questID, runCode)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]models.Objective, len(objectives))
	for _, objective := range objectives {
		byID[objective.ID] = objective
	}

	result := make([]models.Objective, 0, len(completedIDsOrdered))
	for _, id := range completedIDsOrdered {
		if objective, ok := byID[id]; ok {
			result = append(result, objective)
		}
	}
	return result, nil
}

// CountCompletedObjectivesByRun counts reveal-context completions, matching
// GetIncompleteObjectives' definition of "complete".
func (s *RunService) CountCompletedObjectivesByRun(ctx context.Context, questID string) (map[string]int, error) {
	return s.objectiveContextCompletionRepo.CountCompletedObjectivesByRun(ctx, questID, game.ContextObjectiveReveal)
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
