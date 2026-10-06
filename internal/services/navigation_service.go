package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nathanhollows/Rapua/v8/blocks"

	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

var (
	ErrInstanceNotFound = errors.New("instance not found")
	// ErrSectionNotFinishable means the run cannot end that section: its band
	// has no range, its minimum is unmet, or it is finished already.
	ErrSectionNotFinishable = errors.New("section is not finishable")
	// ErrObjectiveNotInQuest means a request named an objective belonging to
	// some other quest. Previewing takes an id from the URL, so the id alone
	// proves nothing about who may see what it points at.
	ErrObjectiveNotInQuest = errors.New("objective does not belong to this quest")
)

type NavigationService struct {
	loader            runStateLoader
	objectiveRepo     repositories.ObjectiveRepository
	sectionFinishRepo repositories.SectionFinishRepository
	teamRepo          repositories.RunRepository
	logger            *slog.Logger
}

// PlayerObjectiveView is what the /objectives list renders: the frontier for
// one run, and the settings the page reads. No blocks: the list shows titles.
type PlayerObjectiveView struct {
	Settings models.QuestSettings
	Frontier navigation.Frontier
	// Complete is true once the quest's root objective is complete, which is
	// the only thing that means the run is finished. An empty available list
	// does not: everything below may simply be waiting on a depends.
	Complete bool
	// Objectives is the quest's whole tree. The frontier is flat by design, and
	// a row in it carries only its parent's ID, so the player view needs this
	// to draw the section a row came out of.
	Objectives []models.Objective
	// HasProof names the objectives whose proof gates them. One without has
	// nothing to do but be opened, which the player view says rather than
	// promising work that is not there.
	HasProof map[string]bool
	// FirstProofBlock names, per objective on offer, the first block in its
	// proof that asks the player for something. The row draws that block's own
	// icon, so a card says what is coming without anyone classifying
	// objectives into kinds.
	FirstProofBlock map[string]string
	// OfferedProof names every block type the frontier asks for, which is what
	// the quest screen's own actions are derived from: a scanner is offered
	// because something on offer can be scanned.
	OfferedProof map[string]bool
}

// Offers reports whether anything the run can work on now asks for this kind of
// block, which is how the quest screen decides what to put under the player's
// thumb.
func (v PlayerObjectiveView) Offers(blockType string) bool {
	return v.OfferedProof[blockType]
}

// NewNavigationService creates a NavigationService.
func NewNavigationService(
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
	sectionFinishRepo repositories.SectionFinishRepository,
	blockRepo repositories.BlockRepository,
	teamRepo repositories.RunRepository,
	logger *slog.Logger,
) *NavigationService {
	return &NavigationService{
		loader: runStateLoader{
			objectiveRepo:                  objectiveRepo,
			objectiveContextCompletionRepo: objectiveContextCompletionRepo,
			sectionFinishRepo:              sectionFinishRepo,
			blockRepo:                      blockRepo,
		},
		objectiveRepo:     objectiveRepo,
		sectionFinishRepo: sectionFinishRepo,
		teamRepo:          teamRepo,
		logger:            logger,
	}
}

// GetPlayerObjectiveView computes the frontier for a run.
func (s *NavigationService) GetPlayerObjectiveView(
	ctx context.Context,
	team *models.Run,
) (*PlayerObjectiveView, error) {
	if err := s.ensureObjectiveTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	objectives, state, complete, err := s.loader.load(ctx, team)
	if err != nil {
		return nil, err
	}

	frontier := navigation.ComputeFrontier(objectives, state, complete)

	// Icons are looked up only for what is on offer. A missing icon costs a
	// row its glyph and nothing else, so a failure here is logged rather than
	// taken out on a page that is otherwise fine.
	icons, offered, err := s.proofIcons(ctx, frontier.Available, blocks.Registry())
	if err != nil {
		s.logger.WarnContext(ctx, "loading proof blocks", "error", err, "run", team.Code)
	}

	return &PlayerObjectiveView{
		Settings:        team.Quest.Settings,
		Frontier:        frontier,
		Complete:        rootComplete(objectives, frontier),
		Objectives:      objectives,
		HasProof:        state.HasProofBlocks,
		FirstProofBlock: icons,
		OfferedProof:    offered,
	}, nil
}

// rootComplete reports whether every parentless objective is complete. A quest
// has one root, so this is that root; a quest whose tree was never built has
// none, and an empty quest is not a finished one.
func rootComplete(objectives []models.Objective, frontier navigation.Frontier) bool {
	roots := 0
	for _, obj := range objectives {
		if obj.ParentID != "" {
			continue
		}
		roots++
		if frontier.StatusOf(obj.ID) != navigation.StatusComplete {
			return false
		}
	}
	return roots > 0
}

// FinishSection records a player ending a section, and reports whether the
// press was the one that recorded it.
//
// The frontier is recomputed from the stored facts rather than trusted from the
// caller: finishing a section can complete an ancestor whose own band is now
// met, so what the player last saw is not what decides this.
func (s *NavigationService) FinishSection(
	ctx context.Context, team *models.Run, objectiveID string,
) (bool, error) {
	objectives, state, complete, err := s.loader.load(ctx, team)
	if err != nil {
		return false, err
	}

	frontier := navigation.ComputeFrontier(objectives, state, complete)
	if frontier.StatusOf(objectiveID) != navigation.StatusFinishable {
		return false, fmt.Errorf("%w: %q", ErrSectionNotFinishable, objectiveID)
	}

	return s.sectionFinishRepo.Insert(ctx, team.Code, objectiveID)
}

// ensureObjectiveTeamRelationsLoaded loads the quest and messages: the player
// handlers render team.Messages for the notification banner.
func (s *NavigationService) ensureObjectiveTeamRelationsLoaded(ctx context.Context, team *models.Run) error {
	if team.Quest.ID == "" {
		if err := s.teamRepo.LoadQuest(ctx, team); err != nil {
			return err
		}
	}
	return s.teamRepo.LoadMessages(ctx, team)
}

// GetPreviewObjectiveView shows one objective as though it were the only thing
// available, for an admin previewing it outside a real run.
func (s *NavigationService) GetPreviewObjectiveView(
	ctx context.Context,
	team *models.Run,
	objectiveID string,
) (*PlayerObjectiveView, error) {
	if err := s.ensureObjectiveTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	objective, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return nil, fmt.Errorf("loading objective: %w", err)
	}
	if objective.QuestID != team.QuestID {
		return nil, fmt.Errorf("%w: objective %q", ErrObjectiveNotInQuest, objectiveID)
	}

	return &PlayerObjectiveView{
		Settings: team.Quest.Settings,
		Frontier: navigation.Frontier{
			Status:    map[string]navigation.Status{objective.ID: navigation.StatusAvailable},
			Available: []models.Objective{*objective},
		},
	}, nil
}
