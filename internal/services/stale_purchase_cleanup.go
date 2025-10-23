package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
)

type StalePurchaseCleanupService struct {
	transactor db.Transactor
	logger     *slog.Logger
}

func NewStalePurchaseCleanupService(
	transactor db.Transactor,
	logger *slog.Logger,
) *StalePurchaseCleanupService {
	return &StalePurchaseCleanupService{
		transactor: transactor,
		logger:     logger,
	}
}

const stalePurchaseDays = 7

// CleanupStalePurchases removes pending/failed purchases older than 7 days.
func (s *StalePurchaseCleanupService) CleanupStalePurchases(ctx context.Context) error {
	cutoffTime := time.Now().AddDate(0, 0, -stalePurchaseDays)

	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.NewDelete().
		Model(&models.CreditPurchase{}).
		Where("status IN (?, ?)",
			models.CreditPurchaseStatusPending,
			models.CreditPurchaseStatusFailed).
		Where("created_at < ?", cutoffTime).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete stale purchases: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Cleaned up stale purchases",
		"deleted_count", rowsAffected,
		"cutoff_date", cutoffTime,
	)

	return nil
}
