package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
)

type FacilitatorService struct {
	repo repositories.FacilitatorTokenRepo
}

func NewFacilitatorService(repo repositories.FacilitatorTokenRepo) *FacilitatorService {
	return &FacilitatorService{repo: repo}
}

// Generate a secure random token.
func (s *FacilitatorService) generateToken() string {
	size := 32
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // Should never happen
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
}

// CreateFacilitatorToken generates and stores a facilitator access token.
func (s *FacilitatorService) CreateFacilitatorToken(
	ctx context.Context,
	instanceID string,
	locations []string,
	duration time.Duration,
) (string, error) {
	token := s.generateToken()
	expiry := time.Now().Add(duration)

	newToken := models.FacilitatorToken{
		Token:      token,
		InstanceID: instanceID,
		Locations:  locations,
		ExpiresAt:  expiry,
	}

	err := s.repo.SaveToken(ctx, newToken)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ValidateToken checks if a token is valid and not expired.
func (s *FacilitatorService) ValidateToken(ctx context.Context, token string) (*models.FacilitatorToken, error) {
	facToken, err := s.repo.GetToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("fetching token: %w", err)
	}

	// Check expiration
	if time.Now().After(facToken.ExpiresAt) {
		return nil, errors.New("token has expired")
	}

	return facToken, nil
}

// CleanupExpiredTokens removes all expired facilitator tokens.
func (s *FacilitatorService) CleanupExpiredTokens(ctx context.Context) error {
	return s.repo.CleanUpExpiredTokens(ctx)
}
