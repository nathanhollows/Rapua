package services_test

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFacilitatorService(t *testing.T) (services.FacilitatorService, func()) {
	dbc, cleanup := setupDB(t)

	repo := repositories.NewFacilitatorTokenRepo(dbc)
	service := services.NewFacilitatorService(repo)
	return *service, cleanup
}
func TestFacilitatorService_CreateAndValidateToken(t *testing.T) {
	service, cleanup := setupFacilitatorService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a new facilitator token
	token, err := service.CreateFacilitatorToken(ctx, "game123", []string{"Park", "Tower"}, 24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated token
	facToken, err := service.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.NotNil(t, facToken)
	assert.Equal(t, "game123", facToken.InstanceID)
	assert.ElementsMatch(t, []string{"Park", "Tower"}, facToken.Locations)
}

func TestFacilitatorService_ExpiredToken(t *testing.T) {
	service, cleanup := setupFacilitatorService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a token that expires immediately
	token, err := service.CreateFacilitatorToken(ctx, "gameExpired", []string{"Lab"}, -1*time.Second)
	require.NoError(t, err)

	// Validate expired token
	facToken, err := service.ValidateToken(ctx, token)
	require.Error(t, err)
	assert.Nil(t, facToken)
}

func TestFacilitatorService_CleanupExpiredTokens(t *testing.T) {
	service, cleanup := setupFacilitatorService(t)
	defer cleanup()
	ctx := context.Background()

	// Create expired token
	token, err := service.CreateFacilitatorToken(ctx, "gameX", []string{"Castle"}, -24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Create valid token
	validToken, _ := service.CreateFacilitatorToken(ctx, "gameY", []string{"Castle"}, 24*time.Hour)

	// Cleanup expired tokens
	err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	// Check expired token is gone
	expiredToken, err := service.ValidateToken(ctx, "gameX")
	require.Error(t, err)
	assert.Nil(t, expiredToken)

	// Check valid token still exists
	validTokenData, err := service.ValidateToken(ctx, validToken)
	require.NoError(t, err)
	assert.NotNil(t, validTokenData)
}
