package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/helpers"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/uptrace/bun"
)

type TeamService interface {
	// AddTeams adds teams to the database
	AddTeams(ctx context.Context, instanceID string, count int) ([]models.Team, error)

	// FindAll returns all teams for an instance
	FindAll(ctx context.Context, instanceID string) ([]models.Team, error)
	// FindTeamByCode returns a team by code
	FindTeamByCode(ctx context.Context, code string) (*models.Team, error)
	// GetTeamActivityOverview returns a list of teams and their activity
	GetTeamActivityOverview(ctx context.Context, instanceID string, locations []models.Location) ([]TeamActivity, error)

	// Update updates a team in the database
	Update(ctx context.Context, team *models.Team) error
	// AwardPoints awards points to a team
	AwardPoints(ctx context.Context, team *models.Team, points int, reason string) error
	// Reset wipes a team's progress for re-use
	Reset(ctx context.Context, instanceID string, teamCodes []string) error

	// Delete removes a team from the database
	Delete(ctx context.Context, instanceID string, teamCode string) error
	// DeleteByInstanceID removes all teams for a specific instance
	DeleteByInstanceID(ctx context.Context, tx *bun.Tx, instanceID string) error

	// LoadRelation loads relations for a team
	LoadRelation(ctx context.Context, team *models.Team, relation string) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Team) error
}

type teamService struct {
	transactor     db.Transactor
	teamRepo       repositories.TeamRepository
	checkInRepo    repositories.CheckInRepository
	blockStateRepo repositories.BlockStateRepository
	locationRepo   repositories.LocationRepository
	batchSize      int
}

// NewTeamService creates a new TeamService.
func NewTeamService(transactor db.Transactor,
	tr repositories.TeamRepository,
	cr repositories.CheckInRepository,
	bsr repositories.BlockStateRepository,
	lr repositories.LocationRepository,
) TeamService {
	return &teamService{
		transactor:     transactor,
		teamRepo:       tr,
		checkInRepo:    cr,
		blockStateRepo: bsr,
		locationRepo:   lr,
		batchSize:      100,
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
func (s *teamService) containsCode(teams []models.Team, code string) bool {
	for _, team := range teams {
		if team.Code == code {
			return true
		}
	}
	return false
}

// AddTeams generates and inserts teams in batches, retrying if unique constraint errors occur.
func (s *teamService) AddTeams(ctx context.Context, instanceID string, count int) ([]models.Team, error) {
	var newTeams []models.Team
	for i := 0; i < count; i += s.batchSize {
		size := min(s.batchSize, count-i)
		teams := make([]models.Team, 0, size)

		for j := 0; j < size; j++ {
			var team models.Team
			for {
				// TODO: Remove magic number
				code := helpers.NewCode(4)
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
func (s *teamService) FindAll(ctx context.Context, instanceID string) ([]models.Team, error) {
	return s.teamRepo.FindAll(ctx, instanceID)
}

// FindTeamByCode returns a team by code.
func (s *teamService) FindTeamByCode(ctx context.Context, code string) (*models.Team, error) {
	return s.teamRepo.GetByCode(ctx, code)
}

// GetTeamActivityOverview returns a list of teams and their activity.
func (s *teamService) GetTeamActivityOverview(ctx context.Context, instanceID string, locations []models.Location) ([]TeamActivity, error) {
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
func (s *teamService) Update(ctx context.Context, team *models.Team) error {
	return s.teamRepo.Update(ctx, team)
}

// AwardPoints awards points to a team.
func (s *teamService) AwardPoints(ctx context.Context, team *models.Team, points int, _ string) error {
	team.Points += points
	return s.teamRepo.Update(ctx, team)
}

// Reset wipes a team's progress for re-use.
func (s *teamService) Reset(ctx context.Context, instanceID string, teamCodes []string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction: %w", rollbackErr)
		}
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				fmt.Printf("rolling back transaction: %v\n", rollbackErr)
			}
			panic(p)
		}
	}()

	err = s.teamRepo.Reset(ctx, tx, instanceID, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("resetting team: rollback failed: %w", rollbackErr)
		}
		return fmt.Errorf("resetting team: %w", err)
	}

	err = s.checkInRepo.DeleteByTeamCodes(ctx, tx, instanceID, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting check ins: %w", err)
	}

	err = s.blockStateRepo.DeleteByTeamCodes(ctx, tx, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting block states: %w", err)
	}

	err = s.locationRepo.UpdateStatistics(ctx, tx, instanceID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("updating location statistics: %w", err)
	}

	return tx.Commit()
}

// Delete removes a team from the database.
func (s *teamService) Delete(ctx context.Context, instanceID string, teamCode string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction: %w", rollbackErr)
		}
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				fmt.Printf("rolling back transaction: %v\n", rollbackErr)
			}
			panic(p)
		}
	}()

	err = s.teamRepo.Delete(ctx, tx, instanceID, teamCode)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting team: %w", err)
	}

	err = s.checkInRepo.DeleteByTeamCodes(ctx, tx, instanceID, []string{teamCode})
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting check ins: %w", err)
	}

	err = s.blockStateRepo.DeleteByTeamCodes(ctx, tx, []string{teamCode})
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting block states: %w", err)
	}

	err = s.locationRepo.UpdateStatistics(ctx, tx, instanceID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("updating location statistics: %w", err)
	}

	return tx.Commit()
}

// DeleteByInstanceID removes all teams for a specific instance.
func (s *teamService) DeleteByInstanceID(ctx context.Context, tx *bun.Tx, instanceID string) error {
	err := s.teamRepo.DeleteByInstanceID(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("deleting teams by instance ID: %w", err)
	}

	return nil
}

// LoadRelation loads the specified relation for a team.
func (s *teamService) LoadRelation(ctx context.Context, team *models.Team, relation string) error {
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
func (s *teamService) LoadRelations(ctx context.Context, team *models.Team) error {
	err := s.teamRepo.LoadRelations(ctx, team)
	if err != nil {
		return err
	}

	return nil
}
