package repositories

import (
	"context"
	"time"

	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/uptrace/bun"
)

type TeamStartLogRepository struct {
	db *bun.DB
}

func NewTeamStartLogRepository(db *bun.DB) *TeamStartLogRepository {
	return &TeamStartLogRepository{
		db: db,
	}
}

// GetByUserID returns all team start logs for a user.
func (r *TeamStartLogRepository) GetByUserID(ctx context.Context, userID string) ([]models.TeamStartLog, error) {
	var logs []models.TeamStartLog
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
func (r *TeamStartLogRepository) GetByUserIDWithTimeframe(
	ctx context.Context,
	userID string,
	startTime, endTime time.Time,
) ([]models.TeamStartLog, error) {
	var logs []models.TeamStartLog
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

// GetByUserIDAndInstanceID returns team start logs for a user and specific instance.
func (r *TeamStartLogRepository) GetByUserIDAndInstanceID(
	ctx context.Context,
	userID, instanceID string,
) ([]models.TeamStartLog, error) {
	var logs []models.TeamStartLog
	err := r.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Where("instance_id = ?", instanceID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByUserIDAndInstanceIDWithTimeframe returns team start logs for a user and instance within a timeframe.
func (r *TeamStartLogRepository) GetByUserIDAndInstanceIDWithTimeframe(
	ctx context.Context,
	userID, instanceID string,
	startTime, endTime time.Time,
) ([]models.TeamStartLog, error) {
	var logs []models.TeamStartLog
	err := r.db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		Where("instance_id = ?", instanceID).
		Where("created_at >= ?", startTime).
		Where("created_at <= ?", endTime).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// Create saves a new team start log entry.
func (r *TeamStartLogRepository) CreateWithTx(ctx context.Context, tx *bun.Tx, log *models.TeamStartLog) error {
	_, err := tx.NewInsert().Model(log).Exec(ctx)
	return err
}

// DeleteByUserID deletes all team start logs for a user within a transaction.
func (r *TeamStartLogRepository) DeleteByUserID(ctx context.Context, tx *bun.Tx, userID string) error {
	_, err := tx.NewDelete().
		Model(&models.TeamStartLog{}).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}
