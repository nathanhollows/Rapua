package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

type ObjectiveService interface {
	// CreateObjective writes a new objective under parentID, appended after
	// whatever is already there, and parked as a draft so it is not shown to
	// players before its author is ready. An empty parentID creates the quest's
	// root, which only a quest without one may do, and which is never a draft.
	CreateObjective(ctx context.Context, questID, parentID, title string) (models.Objective, error)
	GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error)
	FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error)
	UpdateObjective(ctx context.Context, objective *models.Objective, data ObjectiveUpdateData) error
	// FindTree returns a quest's objectives with each parent ahead of its children.
	FindTree(ctx context.Context, questID string) ([]models.Objective, error)
	// FindRoot returns the quest's root objective, the node everything hangs from.
	FindRoot(ctx context.Context, questID string) (*models.Objective, error)
	// FindChildren returns one objective's direct children in position order.
	FindChildren(ctx context.Context, questID, parentID string) ([]models.Objective, error)
	// Reposition moves an objective under newParentID at index newPosition.
	// Both must belong to questID: a drag-and-drop caller can't reach across quests.
	Reposition(ctx context.Context, questID, objectiveID, newParentID string, newPosition int) error
}

// ErrParkingBreaksBand is returned when parking an objective would leave its
// parent needing more children than remain in play, so the parent could never
// complete and everything after it would stay locked.
var ErrParkingBreaksBand = errors.New("parking this objective would leave its section unable to complete")

// ErrCannotDraftRoot is returned when an edit would park a quest's root. Lint
// refuses the same shape in a document (ROOT_DRAFT).
var ErrCannotDraftRoot = errors.New("the root objective cannot be a draft")

// ErrInvalidRouting mirrors lint's own rejection of a bad routing value, for
// the one mutation path that bypasses lint.
var ErrInvalidRouting = errors.New("invalid routing")

// ErrInvalidBand mirrors lint's BAND_MIN_EXCEEDS_MAX rule.
var ErrInvalidBand = errors.New("invalid completion band")

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

// applyObjectiveSettings applies every setting the update data names, leaving
// everything else alone. It is split out of UpdateObjective to keep that
// function's branching readable.
func applyObjectiveSettings(objective *models.Objective, data ObjectiveUpdateData) (bool, error) {
	changed := false

	if data.Routing != nil && *data.Routing != string(objective.Routing) {
		if *data.Routing != "" {
			if _, err := models.ParseRouteStrategy(*data.Routing); err != nil {
				return false, fmt.Errorf("%w: %q", ErrInvalidRouting, *data.Routing)
			}
		}
		objective.Routing = models.RouteStrategy(*data.Routing)
		changed = true
	}

	if data.MaxNext != nil && *data.MaxNext != objective.MaxNext {
		if *data.MaxNext < 0 {
			return false, fmt.Errorf("%w: max_next cannot be negative", ErrInvalidBand)
		}
		objective.MaxNext = *data.MaxNext
		changed = true
	}

	if bandChanged, err := applyObjectiveBand(objective, data); err != nil {
		return false, err
	} else if bandChanged {
		changed = true
	}

	if data.FinishLabel != nil && *data.FinishLabel != objective.FinishLabel {
		objective.FinishLabel = *data.FinishLabel
		changed = true
	}

	if data.Color != nil && *data.Color != objective.Color {
		objective.Color = *data.Color
		changed = true
	}

	if data.Depends != nil && !slices.Equal(data.Depends, []string(objective.Depends)) {
		objective.Depends = data.Depends
		changed = true
	}

	return changed, nil
}

// applyObjectiveBand validates the two bounds together, because min > max only
// means something once both are known. A bound the update does not name checks
// the one it does against the objective's current value.
func applyObjectiveBand(objective *models.Objective, data ObjectiveUpdateData) (bool, error) {
	if data.ChildrenMin == nil && data.ChildrenMax == nil {
		return false, nil
	}
	minBound := objective.ChildrenMin
	if data.ChildrenMin != nil {
		minBound = data.ChildrenMin
	}
	maxBound := objective.ChildrenMax
	if data.ChildrenMax != nil {
		maxBound = data.ChildrenMax
	}

	if minBound != nil && *minBound < 0 {
		return false, fmt.Errorf("%w: children_min cannot be negative", ErrInvalidBand)
	}
	if maxBound != nil && *maxBound < 0 {
		return false, fmt.Errorf("%w: children_max cannot be negative", ErrInvalidBand)
	}
	if minBound != nil && maxBound != nil && *minBound > *maxBound {
		return false, fmt.Errorf("%w: min %d exceeds max %d", ErrInvalidBand, *minBound, *maxBound)
	}

	objective.ChildrenMin = minBound
	objective.ChildrenMax = maxBound
	return true, nil
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
		available, err := s.objectiveRepo.SlugAvailable(ctx, questID, candidate, excludeID)
		if err != nil {
			return "", fmt.Errorf("checking slug availability: %w", err)
		}
		if available {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%s", base, uuid.New().String()[:6])
	}
	return "", fmt.Errorf("could not generate unique slug for %q after %d attempts", title, maxAttempts)
}

func (s objectiveService) CreateObjective(
	ctx context.Context, questID, parentID, title string,
) (models.Objective, error) {
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
		QuestID:  questID,
		ParentID: parentID,
		Title:    title,
		Slug:     slug,
		// New objectives arrive parked. A quest can be live while it is being
		// built, and an objective that appears to players the moment it is named
		// is one an author has to finish in a hurry. The root is the exception:
		// it is the quest rather than a place in it, and drafting it would take
		// the whole game out of play.
		Draft: parentID != "",
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

// checkParkingLeavesBandSatisfiable refuses a park that would put an
// objective's parent past the point of ever completing.
//
// A band counts published children, so parking one lowers the number a section
// can ever reach. Where an author wrote an explicit minimum, that minimum can
// end up above what is left, and the section then cannot complete however much
// a run does: everything after it stays locked. Lint says the same thing about
// a document (BAND_OUT_OF_RANGE), but the toggle is the one mutation path lint
// never sees.
func (s objectiveService) checkParkingLeavesBandSatisfiable(
	ctx context.Context, objective *models.Objective,
) error {
	parent, err := s.objectiveRepo.GetByID(ctx, objective.ParentID)
	if err != nil {
		return fmt.Errorf("loading parent to check its band: %w", err)
	}
	if parent.ChildrenMin == nil {
		return nil
	}

	siblings, err := s.objectiveRepo.FindChildren(ctx, objective.QuestID, objective.ParentID)
	if err != nil {
		return fmt.Errorf("loading siblings to check the band: %w", err)
	}

	remaining := 0
	for _, sibling := range siblings {
		if sibling.ID != objective.ID && !sibling.Draft {
			remaining++
		}
	}
	if remaining < *parent.ChildrenMin {
		return fmt.Errorf("%w: %q needs %d of its children and would have %d left in play",
			ErrParkingBreaksBand, parent.Title, *parent.ChildrenMin, remaining)
	}
	return nil
}

func (s objectiveService) GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error) {
	objective, err := s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, slug)
	if err != nil {
		return nil, fmt.Errorf("finding objective by slug: %w", err)
	}
	return objective, nil
}

func (s objectiveService) FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error) {
	objectives, err := s.objectiveRepo.FindByQuestID(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("finding objectives: %w", err)
	}
	return objectives, nil
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

	if data.Draft != nil && *data.Draft != objective.Draft {
		// The root is the quest rather than a place in it, so parking it takes
		// every objective out of play at once, and the builder does not list
		// the root, which leaves nothing on screen explaining why.
		if *data.Draft && objective.ParentID == "" {
			return fmt.Errorf("%w: %q", ErrCannotDraftRoot, objective.ID)
		}
		if *data.Draft {
			if err := s.checkParkingLeavesBandSatisfiable(ctx, objective); err != nil {
				return err
			}
		}
		objective.Draft = *data.Draft
		update = true
	}

	settingsChanged, err := applyObjectiveSettings(objective, data)
	if err != nil {
		return err
	}
	update = update || settingsChanged

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

func (s objectiveService) FindRoot(ctx context.Context, questID string) (*models.Objective, error) {
	return s.objectiveRepo.FindRoot(ctx, questID)
}

func (s objectiveService) FindTree(ctx context.Context, questID string) ([]models.Objective, error) {
	return s.objectiveRepo.FindTreeByQuestID(ctx, questID)
}

func (s objectiveService) FindChildren(
	ctx context.Context, questID, parentID string,
) ([]models.Objective, error) {
	return s.objectiveRepo.FindChildren(ctx, questID, parentID)
}

func (s objectiveService) Reposition(
	ctx context.Context, questID, objectiveID, newParentID string, newPosition int,
) error {
	objective, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return fmt.Errorf("finding objective: %w", err)
	}
	if objective.QuestID != questID {
		return fmt.Errorf("%w: %q", ErrObjectiveNotInQuest, objectiveID)
	}

	newParent, err := s.objectiveRepo.GetByID(ctx, newParentID)
	if err != nil {
		return fmt.Errorf("finding new parent: %w", err)
	}
	if newParent.QuestID != questID {
		return fmt.Errorf("%w: %q", ErrObjectiveNotInQuest, newParentID)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	if err := s.objectiveRepo.Reposition(ctx, tx, objectiveID, newParentID, newPosition); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("repositioning objective: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
