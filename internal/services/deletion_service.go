package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

// DeleteService orchestrates deletion operations where multiple repositories are involved.
type DeleteService struct {
	transactor     db.Transactor
	blockRepo      repositories.BlockRepository
	blockStateRepo repositories.BlockStateRepository
}

// NewDeleteService creates a new DeletionService with the provided transactor.
func NewDeleteService(transactor db.Transactor, blockRepo repositories.BlockRepository, blockStateRepo repositories.BlockStateRepository) *DeleteService {
	return &DeleteService{transactor: transactor, blockRepo: blockRepo, blockStateRepo: blockStateRepo}
}

// DeleteBlock deletes a block by its ID.
func (s *DeleteService) DeleteBlock(ctx context.Context, blockID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Ensure rollback on failure
	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			log.Printf("recovered from panic, rolling back transaction: %v", err)
			panic(p)
		}
	}()

	if err := s.blockRepo.Delete(ctx, tx, blockID); err != nil {
		err := tx.Rollback()
		if err != nil {
			return fmt.Errorf("deleting block: transaction rollback: %w", err)
		}
		return fmt.Errorf("deleting block: %w", err)
	}

	if err := s.blockStateRepo.DeleteByBlockID(ctx, tx, blockID); err != nil {
		err := tx.Rollback()
		if err != nil {
			return fmt.Errorf("deleting block state: transaction rollback: %w", err)
		}
		return fmt.Errorf("deleting block state: %w", err)
	}

	return tx.Commit()
}
