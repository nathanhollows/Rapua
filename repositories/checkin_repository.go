package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun"
)

type CheckInRepository interface {
	// FindCheckInByTeamAndLocation finds a check-in by team and location
	FindCheckInByTeamAndLocation(ctx context.Context, teamCode string, locationID string) (*models.CheckIn, error)

	// LogCheckIn logs a new check-in for a team at a location
	LogCheckIn(ctx context.Context, team models.Team, location models.Location, mustCheckOut bool, validationRequired bool) (models.CheckIn, error)
	// LogCheckOut checks out a team from a location
	LogCheckOut(ctx context.Context, team *models.Team, location *models.Location) (models.CheckIn, error)

	// Update updates an existing check-in
	Update(ctx context.Context, checkIn *models.CheckIn) error

	// DeleteByTeamCodes deletes all check-ins for the given teams
	DeleteByTeamCodes(ctx context.Context, tx *bun.Tx, instanceID string, teamCodes []string) error
}

type checkInRepository struct {
	db *bun.DB
}

func NewCheckInRepository(db *bun.DB) CheckInRepository {
	return &checkInRepository{
		db: db,
	}
}

func (r *checkInRepository) FindCheckInByTeamAndLocation(ctx context.Context, teamCode string, locationID string) (*models.CheckIn, error) {
	var checkIn models.CheckIn
	err := r.db.NewSelect().Model(&checkIn).Where("team_code = ? AND location_id = ?", teamCode, locationID).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding check in by team and location: %w", err)
	}
	return &checkIn, nil
}

func (r *checkInRepository) Update(ctx context.Context, checkIn *models.CheckIn) error {
	_, err := r.db.NewUpdate().Model(checkIn).WherePK().Exec(ctx)
	return err
}

// LogCheckIn logs a check in for a team at a location.
func (r *checkInRepository) LogCheckIn(ctx context.Context, team models.Team, location models.Location, mustCheckOut bool, validationRequired bool) (models.CheckIn, error) {
	scan := &models.CheckIn{
		TeamID:          team.Code,
		LocationID:      location.ID,
		InstanceID:      team.InstanceID,
		TimeIn:          time.Now().UTC(),
		MustCheckOut:    mustCheckOut,
		Points:          location.Points,
		BlocksCompleted: !validationRequired,
	}
	var err error
	if scan.CreatedAt.IsZero() {
		_, err = r.db.NewInsert().Model(scan).Exec(ctx)
	} else {
		_, err = r.db.NewUpdate().Model(scan).WherePK().Exec(ctx)
	}
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("saving scan: %w", err)
	}

	return *scan, nil
}

// LogCheckOut logs a check out for a team at a location.
func (r *checkInRepository) LogCheckOut(ctx context.Context, team *models.Team, location *models.Location) (models.CheckIn, error) {
	if team == nil {
		return models.CheckIn{}, errors.New("team is required")
	}

	if location == nil {
		return models.CheckIn{}, errors.New("location is required")
	}

	if len(team.CheckIns) == 0 {
		return models.CheckIn{}, errors.New("no check ins found for team")
	}

	var checkIn *models.CheckIn
	for i := range team.CheckIns {
		if team.CheckIns[i].LocationID == location.ID {
			checkIn = &team.CheckIns[i]
			break
		}
	}

	if checkIn == nil {
		return models.CheckIn{}, errors.New("check in not found")
	}

	checkIn.TimeOut = time.Now().UTC()
	checkIn.MustCheckOut = false
	err := r.Update(ctx, checkIn)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("updating check in: %w", err)
	}

	return *checkIn, nil
}

// DeleteByTeamCodes deletes all check-ins for the given teams.
func (r *checkInRepository) DeleteByTeamCodes(ctx context.Context, tx *bun.Tx, instanceID string, teamCodes []string) error {
	_, err := tx.NewDelete().
		Model(&models.CheckIn{}).
		Where("instance_id = ? AND team_code IN (?)", instanceID, bun.In(teamCodes)).
		Exec(ctx)
	return err
}
