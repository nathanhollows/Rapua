package repositories

import (
	"context"
	"time"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

type FacilitatorTokenRepo struct {
	db *bun.DB
}

func NewFacilitatorTokenRepo(db *bun.DB) FacilitatorTokenRepo {
	return FacilitatorTokenRepo{db: db}
}

// Save a facilitator token.
func (r *FacilitatorTokenRepo) SaveToken(ctx context.Context, token models.FacilitatorToken) error {
	_, err := r.db.NewInsert().Model(&token).Exec(ctx)
	return err
}

// Retrieve a token.
func (r *FacilitatorTokenRepo) GetToken(ctx context.Context, token string) (*models.FacilitatorToken, error) {
	var facToken models.FacilitatorToken
	err := r.db.NewSelect().Model(&facToken).Where("token = ?", token).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &facToken, nil
}

func (r *FacilitatorTokenRepo) CleanUpExpiredTokens(ctx context.Context) error {
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := r.db.NewDelete().
		Model(&models.FacilitatorToken{}).
		Where("expires_at < ?", currentTime).
		Exec(ctx)
	return err
}
