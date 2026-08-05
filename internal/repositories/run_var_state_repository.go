package repositories

import (
	"context"
	"time"

	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

// RunVarStateRepository manages persistent creator-defined variable state per team per instance.
type RunVarStateRepository interface {
	// Upsert sets a variable value, inserting or updating as needed.
	Upsert(ctx context.Context, runCode, questID, varName, varValue string) error
	// GetAll returns all var states for a team in an instance as a map[varName]varValue.
	GetAll(ctx context.Context, runCode, questID string) (map[string]string, error)
	// DeleteByTeamAndInstance removes all vars for a team in an instance (used on reset).
	DeleteByTeamAndInstance(ctx context.Context, tx *bun.Tx, runCode, questID string) error
}

type teamVarStateRepository struct {
	db *bun.DB
}

// NewRunVarStateRepository creates a new RunVarStateRepository.
func NewRunVarStateRepository(db *bun.DB) RunVarStateRepository {
	return &teamVarStateRepository{db: db}
}

// Upsert inserts or updates a variable value for a team in an instance.
func (r *teamVarStateRepository) Upsert(
	ctx context.Context,
	runCode, questID, varName, varValue string,
) error {
	row := &models.RunVarState{
		RunCode:   runCode,
		QuestID:   questID,
		VarName:   varName,
		VarValue:  varValue,
		UpdatedAt: time.Now(),
	}
	_, err := r.db.NewInsert().
		Model(row).
		On("CONFLICT (run_code, quest_id, var_name) DO UPDATE").
		Set("var_value = EXCLUDED.var_value").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// GetAll returns all variable states for a team in an instance as a map.
func (r *teamVarStateRepository) GetAll(
	ctx context.Context,
	runCode, questID string,
) (map[string]string, error) {
	var rows []models.RunVarState
	err := r.db.NewSelect().
		Model(&rows).
		Where("run_code = ?", runCode).
		Where("quest_id = ?", questID).
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
	runCode, questID string,
) error {
	_, err := tx.NewDelete().
		Model(&models.RunVarState{}).
		Where("run_code = ?", runCode).
		Where("quest_id = ?", questID).
		Exec(ctx)
	return err
}
