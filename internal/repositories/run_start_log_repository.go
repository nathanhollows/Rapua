package repositories

import (
	"context"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

type RunStartLogRepository struct {
	db *bun.DB
}

func NewRunStartLogRepository(db *bun.DB) *RunStartLogRepository {
	return &RunStartLogRepository{
		db: db,
	}
}

// GetByUserID returns all team start logs for a user.
func (r *RunStartLogRepository) GetByUserID(ctx context.Context, userID string) ([]models.RunStartLog, error) {
	var logs []models.RunStartLog
	err := r.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByUserIDWithTimeframe returns team start logs for a user within a timeframe.
func (r *RunStartLogRepository) GetByUserIDWithTimeframe(
	ctx context.Context,
	userID string,
	startTime, endTime time.Time,
) ([]models.RunStartLog, error) {
	var logs []models.RunStartLog
	err := r.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Where("created_at >= ?", startTime).
		Where("created_at <= ?", endTime).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByUserIDAndQuestID returns run start logs for a user and specific quest.
func (r *RunStartLogRepository) GetByUserIDAndQuestID(
	ctx context.Context,
	userID, questID string,
) ([]models.RunStartLog, error) {
	var logs []models.RunStartLog
	err := r.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Where("quest_id = ?", questID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByUserIDAndQuestIDWithTimeframe returns run start logs for a user and quest within a timeframe.
func (r *RunStartLogRepository) GetByUserIDAndQuestIDWithTimeframe(
	ctx context.Context,
	userID, questID string,
	startTime, endTime time.Time,
) ([]models.RunStartLog, error) {
	var logs []models.RunStartLog
	err := r.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Where("quest_id = ?", questID).
		Where("created_at >= ?", startTime).
		Where("created_at <= ?", endTime).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// CreateWithTx saves a new team start log entry.
func (r *RunStartLogRepository) CreateWithTx(ctx context.Context, tx *bun.Tx, log *models.RunStartLog) error {
	_, err := tx.NewInsert().Model(log).Exec(ctx)
	return err
}
