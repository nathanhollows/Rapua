package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupClueRepo(t *testing.T) (repositories.ClueRepository, func()) {
	t.Helper()
	db, cleanup := setupDB(t)

	clueRepo := repositories.NewClueRepository(db)
	return clueRepo, cleanup
}

func TestClueRepository_Save(t *testing.T) {
	repo, cleanup := setupClueRepo(t)
	defer cleanup()
	ctx := context.Background()

	clue := &models.Clue{
		ID:         uuid.New().String(),
		InstanceID: "instance-1",
		LocationID: "location-1",
		Content:    "This is a test clue.",
	}

	err := repo.Save(ctx, clue)
	assert.NoError(t, err, "expected no error when saving clue")
}

func TestClueRepository_FindCluesByLocation(t *testing.T) {
	repo, cleanup := setupClueRepo(t)
	defer cleanup()
	ctx := context.Background()

	locationID := "location-1"
	clue1 := &models.Clue{
		InstanceID: "instance-1",
		LocationID: locationID,
		Content:    "Clue 1",
	}
	clue2 := &models.Clue{
		InstanceID: "instance-2",
		LocationID: locationID,
		Content:    "Clue 2",
	}

	// Save clues first
	err := repo.Save(ctx, clue1)
	assert.NoError(t, err, "expected no error when saving clue 1")
	err = repo.Save(ctx, clue2)
	assert.NoError(t, err, "expected no error when saving clue 2")

	clues, err := repo.FindCluesByLocation(ctx, locationID)
	assert.NoError(t, err, "expected no error when finding clues by location")
	assert.Len(t, clues, 2, "expected two clues to be found")
}

func TestClueRepository_DeleteByLocationID(t *testing.T) {
	repo, cleanup := setupClueRepo(t)
	defer cleanup()
	ctx := context.Background()

	locationID := gofakeit.UUID()
	instanceID := gofakeit.UUID()
	for i := 0; i < 3; i++ {
		clue := &models.Clue{
			InstanceID: instanceID,
			LocationID: locationID,
			Content:    gofakeit.Sentence(5),
		}
		err := repo.Save(ctx, clue)
		assert.NoError(t, err, "expected no error when saving clue")
	}

	err := repo.DeleteByLocationID(ctx, locationID)
	assert.NoError(t, err, "expected no error when deleting clues by location ID")

	clues, err := repo.FindCluesByLocation(ctx, locationID)
	assert.NoError(t, err, "expected no error when finding clues by location")
	assert.Len(t, clues, 0, "expected no clues to be found")
}
