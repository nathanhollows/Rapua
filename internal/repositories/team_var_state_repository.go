package repositories

import (
	"context"
	"time"

	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

// TeamVarStateRepository manages persistent creator-defined variable state per team per instance.
type TeamVarStateRepository interface {
	// Upsert sets a variable value, inserting or updating as needed.
	Upsert(ctx context.Context, teamCode, instanceID, varName, varValue string) error
	// GetAll returns all var states for a team in an instance as a map[varName]varValue.
	GetAll(ctx context.Context, teamCode, instanceID string) (map[string]string, error)
	// DeleteByTeamAndInstance removes all vars for a team in an instance (used on reset).
	DeleteByTeamAndInstance(ctx context.Context, tx *bun.Tx, teamCode, instanceID string) error
}

type teamVarStateRepository struct {
	db *bun.DB
}

// NewTeamVarStateRepository creates a new TeamVarStateRepository.
func NewTeamVarStateRepository(db *bun.DB) TeamVarStateRepository {
	return &teamVarStateRepository{db: db}
}

// Upsert inserts or updates a variable value for a team in an instance.
func (r *teamVarStateRepository) Upsert(
	ctx context.Context,
	teamCode, instanceID, varName, varValue string,
) error {
	row := &models.TeamVarState{
		TeamCode:   teamCode,
		InstanceID: instanceID,
		VarName:    varName,
		VarValue:   varValue,
		UpdatedAt:  time.Now(),
	}
	_, err := r.db.NewInsert().
		Model(row).
		On("CONFLICT (team_code, instance_id, var_name) DO UPDATE").
		Set("var_value = EXCLUDED.var_value").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// GetAll returns all variable states for a team in an instance as a map.
func (r *teamVarStateRepository) GetAll(
	ctx context.Context,
	teamCode, instanceID string,
) (map[string]string, error) {
	var rows []models.TeamVarState
	err := r.db.NewSelect().
		Model(&rows).
		Where("team_code = ?", teamCode).
		Where("instance_id = ?", instanceID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[row.VarName] = row.VarValue
	}
	return result, nil
}

// DeleteByTeamAndInstance removes all var states for a team in an instance.
func (r *teamVarStateRepository) DeleteByTeamAndInstance(
	ctx context.Context,
	tx *bun.Tx,
	teamCode, instanceID string,
) error {
	_, err := tx.NewDelete().
		Model(&models.TeamVarState{}).
		Where("team_code = ?", teamCode).
		Where("instance_id = ?", instanceID).
		Exec(ctx)
	return err
}
