package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

type ObjectiveService interface {
	CreateObjective(ctx context.Context, questID, title string) (models.Objective, error)
	GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error)
	UpdateObjective(ctx context.Context, objective *models.Objective, data ObjectiveUpdateData) error
}

type objectiveService struct {
	transactor    db.Transactor
	objectiveRepo repositories.ObjectiveRepository
}

func NewObjectiveService(
	transactor db.Transactor,
	objectiveRepo repositories.ObjectiveRepository,
) ObjectiveService {
	return objectiveService{
		transactor:    transactor,
		objectiveRepo: objectiveRepo,
	}
}

// generateUniqueSlug returns a slug unique within questID, excluding excludeID from conflict checks.
func (s objectiveService) generateUniqueSlug(ctx context.Context, questID, title, excludeID string) (string, error) {
	base := models.Slugify(title)
	if base == "" {
		base = "objective"
	}
	candidate := base
	const maxAttempts = 100
	for range maxAttempts {
		existing, err := s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, candidate)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return candidate, nil
			}
			return "", fmt.Errorf("checking slug availability: %w", err)
		}
		if existing.ID == excludeID {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%s", base, uuid.New().String()[:6])
	}
	return "", fmt.Errorf("could not generate unique slug for %q after %d attempts", title, maxAttempts)
}

func (s objectiveService) CreateObjective(ctx context.Context, questID, title string) (models.Objective, error) {
	if questID == "" {
		return models.Objective{}, errors.New("questID cannot be empty")
	}
	if title == "" {
		return models.Objective{}, errors.New("title cannot be empty")
	}

	slug, err := s.generateUniqueSlug(ctx, questID, title, "")
	if err != nil {
		return models.Objective{}, fmt.Errorf("generating slug: %w", err)
	}

	objective := models.Objective{
		QuestID: questID,
		Title:   title,
		Slug:    slug,
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return models.Objective{}, fmt.Errorf("beginning transaction: %w", err)
	}
	if err := s.objectiveRepo.CreateTx(ctx, tx, &objective); err != nil {
		_ = tx.Rollback()
		return models.Objective{}, fmt.Errorf("saving objective: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.Objective{}, fmt.Errorf("committing transaction: %w", err)
	}

	return objective, nil
}

func (s objectiveService) GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error) {
	objective, err := s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, slug)
	if err != nil {
		return nil, fmt.Errorf("finding objective by slug: %w", err)
	}
	return objective, nil
}

func (s objectiveService) UpdateObjective(
	ctx context.Context,
	objective *models.Objective,
	data ObjectiveUpdateData,
) error {
	update := false

	if data.Title != "" && data.Title != objective.Title {
		objective.Title = data.Title
		newSlug, slugErr := s.generateUniqueSlug(ctx, objective.QuestID, data.Title, objective.ID)
		if slugErr != nil {
			return fmt.Errorf("generating slug: %w", slugErr)
		}
		objective.Slug = newSlug
		update = true
	}

	if !update {
		return nil
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	if err := s.objectiveRepo.UpdateTx(ctx, tx, objective); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("updating objective: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
