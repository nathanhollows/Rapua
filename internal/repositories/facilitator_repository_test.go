package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupFacilitatorTokenRepo(t *testing.T) (repositories.FacilitatorTokenRepo, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	facilitatorTokenRepo := repositories.NewFacilitatorTokenRepo(dbc)

	return facilitatorTokenRepo, dbc, cleanup
}

func TestFacilitatorRepo_SaveAndRetrieveToken(t *testing.T) {
	repo, dbc, cleanup := setupFacilitatorTokenRepo(t)
	defer cleanup()

	ctx := context.Background()

	parents := createTestParents(t, dbc)

	token := models.FacilitatorToken{
		Token:      "jsonTest123",
		InstanceID: parents.InstanceID,
		Locations:  []string{parents.LocationID},
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	// Save token
	err := repo.SaveToken(ctx, token)
	require.NoError(t, err)

	// Retrieve token
	retrieved, err := repo.GetToken(ctx, "jsonTest123")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, token.Token, retrieved.Token)
	assert.Equal(t, token.InstanceID, retrieved.InstanceID)
	assert.ElementsMatch(t, token.Locations, retrieved.Locations) // JSON-safe comparison
}
