package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun"
)

type ClueRepository interface {
	// Save saves or updates a clue in the database
	Save(ctx context.Context, c *models.Clue) error

	// FindCluesByLocation returns all clues for a given location
	FindCluesByLocation(ctx context.Context, locationID string) ([]models.Clue, error)

	// Delete removes the clue from the database
	Delete(ctx context.Context, clueID string) error
	// DeleteByLocationID removes all clues for a location
	// When deleting a location, please use DeleteByLocationIDWithTransaction instead
	DeleteByLocationID(ctx context.Context, locationID string) error
	// DeleteByLocationIDWithTransaction removes all clues for a location with a transaction
	DeleteByLocationIDWithTransaction(ctx context.Context, tx *bun.Tx, locationID string) error
}

type clueRepository struct {
	db *bun.DB
}

// NewClueRepository creates a new ClueRepository.
func NewClueRepository(db *bun.DB) ClueRepository {
	return &clueRepository{
		db: db,
	}
}

// Save saves or updates a clue in the database.
func (r *clueRepository) Save(ctx context.Context, c *models.Clue) error {
	if c.InstanceID == "" || c.LocationID == "" {
		return errors.New("instance ID and location ID must be set")
	}
	var err error
	if c.ID == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("generating UUID: %w", err)
		}
		c.ID = id.String()
	}
	_, err = r.db.NewInsert().Model(c).Exec(ctx)
	return err
}

// FindCluesByLocation returns all clues for a given location.
func (r *clueRepository) FindCluesByLocation(ctx context.Context, locationID string) ([]models.Clue, error) {
	clues := []models.Clue{}
	err := r.db.
		NewSelect().
		Model(&clues).
		Where("location_id = ?", locationID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding clues by location: %w", err)
	}
	return clues, nil
}

// Delete removes the clue from the database.
func (r *clueRepository) Delete(ctx context.Context, clueID string) error {
	_, err := r.db.
		NewDelete().
		Model(&models.Clue{ID: clueID}).
		ForceDelete().
		WherePK().
		Exec(ctx)
	return err
}

// DeleteByLocationID removes all clues for a location.
func (r *clueRepository) DeleteByLocationID(ctx context.Context, locationID string) error {
	_, err := r.db.
		NewDelete().
		Model(&models.Clue{}).
		Where("location_id = ?", locationID).
		ForceDelete().
		Exec(ctx)
	return err
}

// DeleteByLocationIDWithTransaction removes all clues for a location with a transaction.
func (r *clueRepository) DeleteByLocationIDWithTransaction(ctx context.Context, tx *bun.Tx, locationID string) error {
	_, err := tx.
		NewDelete().
		Model(&models.Clue{}).
		Where("location_id = ?", locationID).
		ForceDelete().
		Exec(ctx)
	return err
}
