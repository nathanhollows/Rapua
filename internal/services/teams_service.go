package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v4/helpers"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
)

type TeamService struct {
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
) *TeamService {
	return &TeamService{
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
func (s *TeamService) FindAll(ctx context.Context, instanceID string) ([]models.Team, error) {
	return s.teamRepo.FindAll(ctx, instanceID)
}

// GetTeamByCode returns a team by code.
func (s *TeamService) GetTeamByCode(ctx context.Context, code string) (*models.Team, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	return s.teamRepo.GetByCode(ctx, code)
}

// GetTeamActivityOverview returns a list of teams and their activity.
func (s *TeamService) GetTeamActivityOverview(ctx context.Context, instanceID string, locations []models.Location) ([]TeamActivity, error) {
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

func (s *TeamService) StartPlaying(ctx context.Context, teamCode, customTeamName string) error {
	team, err := s.GetTeamByCode(ctx, teamCode)
	if err != nil {
		return ErrTeamNotFound
	}

	// Update team with custom name if provided
	if !team.HasStarted || customTeamName != "" {
		team.Name = customTeamName
		team.HasStarted = true
		err = s.Update(ctx, team)
		if err != nil {
			return fmt.Errorf("updating team: %w", err)
		}
	}

	return nil
}
