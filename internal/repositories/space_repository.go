package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

type SpaceRepository interface {
	// Create assigns an ID when the space has none
	Create(ctx context.Context, space *models.Space) error
	CreateTx(ctx context.Context, tx *bun.Tx, space *models.Space) error
	Update(ctx context.Context, space *models.Space) error
	UpdateTx(ctx context.Context, tx *bun.Tx, space *models.Space) error

	GetByID(ctx context.Context, spaceID string) (*models.Space, error)
	GetByQuestAndSlug(ctx context.Context, questID, slug string) (*models.Space, error)
	GetByPayload(ctx context.Context, payload string) (*models.Space, error)
	// FindByQuest returns a quest's spaces ordered by name
	FindByQuest(ctx context.Context, questID string) ([]models.Space, error)

	Delete(ctx context.Context, spaceID string) error
	// DeleteByQuest takes a transaction because related data goes with it
	DeleteByQuest(ctx context.Context, tx *bun.Tx, questID string) error
}

type spaceRepository struct {
	db *bun.DB
}

func NewSpaceRepository(db *bun.DB) SpaceRepository {
	return &spaceRepository{
		db: db,
	}
}

func (r *spaceRepository) Create(ctx context.Context, space *models.Space) error {
	if space.ID == "" {
		space.ID = uuid.New().String()
	}
	_, err := r.db.NewInsert().Model(space).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating space: %w", err)
	}
	return nil
}

func (r *spaceRepository) CreateTx(ctx context.Context, tx *bun.Tx, space *models.Space) error {
	if space.ID == "" {
		space.ID = uuid.New().String()
	}
	_, err := tx.NewInsert().Model(space).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating space: %w", err)
	}
	return nil
}

func (r *spaceRepository) Update(ctx context.Context, space *models.Space) error {
	if space.ID == "" {
		return errors.New("space ID is required")
	}
	space.UpdatedAt = time.Now().UTC()
	_, err := r.db.NewUpdate().
		Model(space).
		Column("slug", "name", "kind", "geometry", "payload", "mobile", "updated_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating space: %w", err)
	}
	return nil
}

func (r *spaceRepository) UpdateTx(ctx context.Context, tx *bun.Tx, space *models.Space) error {
	if space.ID == "" {
		return errors.New("space ID is required")
	}
	space.UpdatedAt = time.Now().UTC()
	_, err := tx.NewUpdate().
		Model(space).
		Column("slug", "name", "kind", "geometry", "payload", "mobile", "updated_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating space: %w", err)
	}
	return nil
}

func (r *spaceRepository) GetByID(ctx context.Context, spaceID string) (*models.Space, error) {
	var space models.Space
	err := r.db.NewSelect().
		Model(&space).
		Where("id = ?", spaceID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding space: %w", err)
	}
	return &space, nil
}

func (r *spaceRepository) GetByQuestAndSlug(ctx context.Context, questID, slug string) (*models.Space, error) {
	var space models.Space
	err := r.db.NewSelect().
		Model(&space).
		Where("quest_id = ? AND slug = ?", questID, slug).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding space by slug: %w", err)
	}
	return &space, nil
}

// Payloads are stored uppercased so a scan resolves however the reader cased it.
func (r *spaceRepository) GetByPayload(ctx context.Context, payload string) (*models.Space, error) {
	payload = strings.ToUpper(strings.TrimSpace(payload))
	var space models.Space
	err := r.db.NewSelect().
		Model(&space).
		Where("payload = ?", payload).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding space by payload: %w", err)
	}
	return &space, nil
}

func (r *spaceRepository) FindByQuest(ctx context.Context, questID string) ([]models.Space, error) {
	var spaces []models.Space
	err := r.db.NewSelect().
		Model(&spaces).
		Where("quest_id = ?", questID).
		Order("name ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding spaces: %w", err)
	}
	return spaces, nil
}

func (r *spaceRepository) Delete(ctx context.Context, spaceID string) error {
	_, err := r.db.NewDelete().
		Model(&models.Space{ID: spaceID}).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting space: %w", err)
	}
	return nil
}

func (r *spaceRepository) DeleteByQuest(ctx context.Context, tx *bun.Tx, questID string) error {
	_, err := tx.NewDelete().
		Model((*models.Space)(nil)).
		Where("quest_id = ?", questID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting spaces for quest: %w", err)
	}
	return nil
}
