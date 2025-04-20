package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/uptrace/bun"
)

type ShareLinkRepository struct {
	db *bun.DB
}

// NewTeamRepository creates a new TeamRepository.
func NewShareLinkRepository(db *bun.DB) ShareLinkRepository {
	return ShareLinkRepository{
		db: db,
	}
}

// Create saves a new share link in the database.
func (r *ShareLinkRepository) Create(ctx context.Context, link *models.ShareLink) error {
	// Sanity check the link
	if link == nil {
		return fmt.Errorf("link is required")
	}
	if link.TemplateID == "" {
		return fmt.Errorf("template ID is required")
	}

	// If the link has no expiration date and no max uses, we first search for an existing link
	if link.ExpiresAt == (bun.NullTime{}) && link.MaxUses == 0 {
		existingLink := new(models.ShareLink)
		err := r.db.NewSelect().
			Model(existingLink).
			Where("share_link.template_id = ?", link.TemplateID).
			Where("share_link.user_id = ?", link.UserID).
			Where("share_link.expires_at IS NULL AND share_link.max_uses = 0").
			Limit(1).
			Scan(ctx)
		if err != nil && err.Error() != "sql: no rows in result set" {
			return fmt.Errorf("failed to check for existing link: %w", err)
		}
		if existingLink != nil && existingLink.ID != "" {
			link.ID = existingLink.ID
			return nil
		}
	}

	// We always want to generate a new UUID for the link
	link.ID = uuid.New().String()
	link.CreatedAt = time.Now()
	link.UsedCount = 0
	_, err := r.db.NewInsert().Model(link).Exec(ctx)
	return err
}

// GetByID retrieves a share link by its ID.
func (r *ShareLinkRepository) GetByID(ctx context.Context, id string) (*models.ShareLink, error) {
	link := new(models.ShareLink)
	err := r.db.NewSelect().
		Model(link).
		Where("share_link.id = ?", id).
		// Ensure the link is active and has not expired
		Relation("Template").
		Relation("Template.Settings").
		Relation("Template.Locations").
		Relation("Template.Locations.Blocks").
		Relation("Template.Locations.Clues").
		Relation("Template.Locations.Marker").
		Scan(ctx)
	return link, err
}

// Use increments the used count for a share link.
func (r *ShareLinkRepository) Use(ctx context.Context, link *models.ShareLink) error {
	res, err := r.db.NewUpdate().Model(link).
		Set("used_count = used_count + 1").
		Where("expires_at > ? AND (max_uses = 0 OR used_count < max_uses)", time.Now()).
		WherePK().
		Exec(ctx)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("link is expired or has reached its maximum uses")
	}
	return nil
}
