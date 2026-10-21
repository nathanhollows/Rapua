package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nathanhollows/Rapua/v8/blocks"

	"github.com/nathanhollows/Rapua/v8/internal/db"
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
	transactor        db.Transactor
	loader            runStateLoader
	objectiveRepo     repositories.ObjectiveRepository
	sectionFinishRepo repositories.SectionFinishRepository
	teamRepo          repositories.RunRepository
	runEventRepo      repositories.RunEventRepository
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
	// Progress is how far the run has come and how much it still owes.
	Progress navigation.Progress
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
}

// NewNavigationService creates a NavigationService.
func NewNavigationService(
	transactor db.Transactor,
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
	sectionFinishRepo repositories.SectionFinishRepository,
	blockRepo repositories.BlockRepository,
	teamRepo repositories.RunRepository,
	runEventRepo repositories.RunEventRepository,
	runVariableRepo repositories.RunVariableRepository,
	logger *slog.Logger,
) *NavigationService {
	return &NavigationService{
		transactor: transactor,
		loader: runStateLoader{
			objectiveRepo:                  objectiveRepo,
			objectiveContextCompletionRepo: objectiveContextCompletionRepo,
			sectionFinishRepo:              sectionFinishRepo,
			blockRepo:                      blockRepo,
			runVariableRepo:                runVariableRepo,
		},
		objectiveRepo:     objectiveRepo,
		sectionFinishRepo: sectionFinishRepo,
		teamRepo:          teamRepo,
		runEventRepo:      runEventRepo,
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
	icons, err := s.proofIcons(ctx, frontier.Available, blocks.Registry())
	if err != nil {
		s.logger.WarnContext(ctx, "loading proof blocks", "error", err, "run", team.Code)
	}

	return &PlayerObjectiveView{
		Settings:        team.Quest.Settings,
		Frontier:        frontier,
		Complete:        rootComplete(objectives, frontier),
		Progress:        navigation.ComputeProgress(objectives, complete),
		Objectives:      objectives,
		HasProof:        state.HasProofBlocks,
		FirstProofBlock: icons,
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

// rootsComplete asks rootComplete's question of the derived completion map
// instead of the frontier, which is what the write paths hold: completion is a
// pure function of the recorded facts, so one more fact answers it without a
// second load.
func rootsComplete(objectives []models.Objective, complete map[string]bool) bool {
	roots := 0
	for _, obj := range objectives {
		if obj.ParentID != "" {
			continue
		}
		roots++
		if !complete[obj.ID] {
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

	// The insert's conflict clause decides whether this call finished the section,
	// so it is also the one that records it: a double press gives one event.
	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	inserted, err := s.sectionFinishRepo.InsertWithTx(ctx, tx, team.Code, objectiveID)
	if err != nil {
		return false, err
	}
	if !inserted {
		return false, nil
	}

	if err := s.runEventRepo.RecordWithTx(ctx, tx, sectionFinishedEvent(team, objectiveID)); err != nil {
		return false, fmt.Errorf("recording section finish: %w", err)
	}

	// The press may be what tips the root over. The section closing is the only
	// fact that changed, and the derivation is pure, so the answer comes from
	// the state already in hand.
	state.SectionFinished[objectiveID] = true
	if rootsComplete(objectives, navigation.ComputeCompleted(objectives, state)) {
		if err := recordRunCompleted(ctx, tx, s.runEventRepo, team); err != nil {
			return false, fmt.Errorf("recording run completion: %w", err)
		}
	}

	return true, tx.Commit()
}

// RecordObjectiveView notes that a run opened an objective. Callers gate
// reachability first, so a guessed slug never counts as a visit.
func (s *NavigationService) RecordObjectiveView(
	ctx context.Context, team *models.Run, objectiveID string,
) error {
	return s.recordPresence(ctx, team, objectiveViewedEvent(team, objectiveID))
}

// RecordObjectivesListed notes that a run opened the list of objectives.
func (s *NavigationService) RecordObjectivesListed(ctx context.Context, team *models.Run) error {
	return s.recordPresence(ctx, team, objectivesListedEvent(team))
}

// recordPresence skips a finished run: a team browsing its old objectives is
// not playing, and rows past the finish fall outside the run's timeline.
func (s *NavigationService) recordPresence(ctx context.Context, team *models.Run, event *models.RunEvent) error {
	finished, err := s.runEventRepo.RunCompleted(ctx, team.Code)
	if err != nil {
		return fmt.Errorf("checking whether the run has finished: %w", err)
	}
	if finished {
		return nil
	}
	return s.runEventRepo.Record(ctx, event)
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
