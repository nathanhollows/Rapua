package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nathanhollows/Rapua/models"
	"github.com/uptrace/bun"
)

type TeamRepository interface {
	// InsertBatch adds multiple teams to the database
	InsertBatch(ctx context.Context, teams []models.Team) error

	// FindAll returns all teams for an instance
	FindAll(ctx context.Context, instanceID string) ([]models.Team, error)
	// FindAllWithScans returns all teams for an instance with scans
	FindAllWithScans(ctx context.Context, instanceID string) ([]models.Team, error)
	// FindTeamByCode returns a team by its code
	FindTeamByCode(ctx context.Context, code string) (*models.Team, error)

	// Update saves or updates a team in the database
	Update(ctx context.Context, t *models.Team) error

	// Delete removes the team from the database
	Delete(ctx context.Context, instanceID string, teamCode string) error
	// DeleteByInstanceID removes all teams for a specific instance
	DeleteByInstanceID(ctx context.Context, instanceID string) error

	// LoadInstance loads the instance for a team
	LoadInstance(ctx context.Context, team *models.Team) error
	// LoadCheckIns loads the check-ins for a team
	LoadCheckIns(ctx context.Context, team *models.Team) error
	// LoadBlockingLocation loads the blocking location for a team
	LoadBlockingLocation(ctx context.Context, team *models.Team) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Team) error
}

type teamRepository struct {
	db *bun.DB
}

// NewTeamRepository creates a new TeamRepository
func NewTeamRepository(db *bun.DB) TeamRepository {
	return &teamRepository{
		db: db,
	}
}

// Update saves or updates a team in the database
func (r *teamRepository) Update(ctx context.Context, t *models.Team) error {
	_, err := r.db.NewUpdate().Model(t).WherePK().Exec(ctx)
	return err
}

func (r *teamRepository) Delete(ctx context.Context, instanceID string, teamCode string) error {
	_, err := r.db.
		NewDelete().
		Model(&models.Team{}).
		Where("code = ? AND instance_id = ?", teamCode, instanceID).
		Exec(ctx)
	return err
}

func (r *teamRepository) DeleteByInstanceID(ctx context.Context, instanceID string) error {
	_, err := r.db.NewDelete().Model(&models.Team{}).Where("instance_id = ?", instanceID).Exec(ctx)
	return err
}

func (r *teamRepository) FindAll(ctx context.Context, instanceID string) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.NewSelect().
		Model(&teams).
		Where("team.instance_id = ?", instanceID).
		Scan(ctx)
	if err != nil {
		return teams, err
	}
	return teams, nil
}

func (r *teamRepository) FindAllWithScans(ctx context.Context, instanceID string) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.NewSelect().
		Model(&teams).
		Where("team.instance_id = ?", instanceID).
		// Add the scans in the relation order by location_id
		Relation("CheckIns", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("location_id ASC")
		}).
		Scan(ctx)
	if err != nil {
		return teams, err
	}
	return teams, nil
}

// FindTeamByCode returns a team by code
func (r *teamRepository) FindTeamByCode(ctx context.Context, code string) (*models.Team, error) {
	code = strings.ToUpper(code)
	var team models.Team
	err := r.db.NewSelect().Model(&team).Where("team.code = ?", code).
		Relation("Instance").
		Relation("BlockingLocation").
		Relation("CheckIns", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("name ASC")
		}).
		Limit(1).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("FindTeamByCode: %v", err)
	}
	return &team, nil
}

// InsertBatch inserts a batch of teams and returns an error if there's a unique constraint conflict
func (r *teamRepository) InsertBatch(ctx context.Context, teams []models.Team) error {
	_, err := r.db.NewInsert().Model(&teams).Exec(ctx)
	if err != nil && isUniqueConstraintError(err) {
		return errors.New("unique constraint error")
	}
	return err
}

// isUniqueConstraintError checks if an error is due to a unique constraint violation
func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unique constraint")
}

func (r *teamRepository) LoadInstance(ctx context.Context, team *models.Team) error {
	query := r.db.NewSelect().
		Model(&team.Instance).
		Where("id = ?", team.InstanceID).
		WherePK()

	if team.Instance.Settings.InstanceID == "" {
		query = query.Relation("Settings")
	}

	if len(team.Instance.Locations) == 0 {
		query = query.Relation("Locations.Blocks")
	}

	return query.Scan(ctx)
}

func (r *teamRepository) LoadCheckIns(ctx context.Context, team *models.Team) error {
	// Only load the scans if they are not already loaded
	err := r.db.NewSelect().Model(&team.CheckIns).
		Where("team_code = ?", team.Code).
		Relation("Location").
		Order("time_in DESC").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("LoadCheckIns: %v", err)
	}
	return nil
}

func (r *teamRepository) LoadBlockingLocation(ctx context.Context, team *models.Team) error {
	if team.MustCheckOut == "" || team.BlockingLocation.ID != "" {
		return nil
	}
	err := r.db.NewSelect().Model(&team.BlockingLocation).
		Where("ID = ?", team.MustCheckOut).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("LoadBlockingLocation: %v", err)
	}
	return nil
}

func (r *teamRepository) LoadRelations(ctx context.Context, team *models.Team) error {
	err := r.LoadInstance(ctx, team)
	if err != nil {
		return err
	}

	err = r.LoadCheckIns(ctx, team)
	if err != nil {
		return err
	}

	err = r.LoadBlockingLocation(ctx, team)
	if err != nil {
		return err
	}

	return nil
}
