package services

import (
	"context"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v3/helpers"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
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

	// LoadRelation loads relations for a team
	LoadRelation(ctx context.Context, team *models.Team, relation string) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Team) error
}

type teamService struct {
	teamRepo       repositories.TeamRepository
	checkInRepo    repositories.CheckInRepository
	blockStateRepo repositories.BlockStateRepository
	locationRepo   repositories.LocationRepository
	batchSize      int
}

// NewTeamService creates a new TeamService.
func NewTeamService(
	tr repositories.TeamRepository,
	cr repositories.CheckInRepository,
	bsr repositories.BlockStateRepository,
	lr repositories.LocationRepository,
) TeamService {
	return &teamService{
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

// LoadRelation loads the specified relation for a team.
// Relations can be "Instance", "Scans", "BlockingLocation", or "Messages".
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
