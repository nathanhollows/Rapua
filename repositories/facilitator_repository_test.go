package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFacilitatorTokenRepo(t *testing.T) (repositories.FacilitatorTokenRepo, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	facilitatorTokenRepo := repositories.NewFacilitatorTokenRepo(dbc)

	return facilitatorTokenRepo, cleanup
}

func TestFacilitatorRepo_SaveAndRetrieveToken(t *testing.T) {
	repo, cleanup := setupFacilitatorTokenRepo(t)
	defer cleanup()

	ctx := context.Background()

	token := models.FacilitatorToken{
		Token:      "jsonTest123",
		InstanceID: "instanceX",
		Locations:  []string{gofakeit.UUID(), gofakeit.UUID()},
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
