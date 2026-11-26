package services_test

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupFacilitatorService(t *testing.T) (services.FacilitatorService, *bun.DB, func()) {
	dbc, cleanup := setupDB(t)

	repo := repositories.NewFacilitatorTokenRepo(dbc)
	service := services.NewFacilitatorService(repo)
	return *service, dbc, cleanup
}
func TestFacilitatorService_CreateAndValidateToken(t *testing.T) {
	service, dbc, cleanup := setupFacilitatorService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a valid quest to satisfy FK constraint: facilitator_tokens.quest_id → quests.id
	parents := createTestParents(t, dbc)

	// Create a new facilitator token
	token, err := service.CreateFacilitatorToken(ctx, parents.QuestID, []string{"Park", "Tower"}, 24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated token
	facToken, err := service.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.NotNil(t, facToken)
	assert.Equal(t, parents.QuestID, facToken.QuestID)
	assert.ElementsMatch(t, []string{"Park", "Tower"}, facToken.Objectives)
}

func TestFacilitatorService_ExpiredToken(t *testing.T) {
	service, dbc, cleanup := setupFacilitatorService(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	// Create a token that expires immediately
	token, err := service.CreateFacilitatorToken(ctx, parents.QuestID, []string{"Lab"}, -1*time.Second)
	require.NoError(t, err)

	// Validate expired token
	facToken, err := service.ValidateToken(ctx, token)
	require.Error(t, err)
	assert.Nil(t, facToken)
}

func TestFacilitatorService_CleanupExpiredTokens(t *testing.T) {
	service, dbc, cleanup := setupFacilitatorService(t)
	defer cleanup()
	ctx := context.Background()

	parentsX := createTestParents(t, dbc)
	parentsY := createTestParents(t, dbc)

	// Create expired token
	token, err := service.CreateFacilitatorToken(ctx, parentsX.QuestID, []string{"Castle"}, -24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Create valid token
	validToken, _ := service.CreateFacilitatorToken(ctx, parentsY.QuestID, []string{"Castle"}, 24*time.Hour)

	// Cleanup expired tokens
	err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	// Check expired token is gone
	expiredToken, err := service.ValidateToken(ctx, token)
	require.Error(t, err)
	assert.Nil(t, expiredToken)

	// Check valid token still exists
	validTokenData, err := service.ValidateToken(ctx, validToken)
	require.NoError(t, err)
	assert.NotNil(t, validTokenData)
}
