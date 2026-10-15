package services

import (
	"context"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// RunSummary is one run's line on the runs list.
type RunSummary struct {
	// Done and Total exclude ruled-out leaves, so a run that finished a
	// cut-short branch reads complete rather than part-done.
	Done   int
	Total  int
	Status TeamStatus
}

// GetQuestRunSummaries loads the quest-wide data once: calling GetRunProgress
// per run would re-read the tree and block owners per player.
func (s *RunService) GetQuestRunSummaries(ctx context.Context, questID string) (map[string]RunSummary, error) {
	runs, err := s.teamRepo.FindAll(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("finding runs: %w", err)
	}

	all, err := s.objectiveRepo.FindTreeByQuestID(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("finding objectives: %w", err)
	}
	inPlay := navigation.InPlay(all)

	completed, err := s.objectiveContextCompletionRepo.FindCompletedByQuest(
		ctx, questID, game.ContextObjectiveProof,
	)
	if err != nil {
		return nil, fmt.Errorf("finding completed objectives: %w", err)
	}
	closed, err := s.sectionFinishRepo.FindFinishedByQuest(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("finding finished sections: %w", err)
	}

	finishes, err := s.runEventRepo.LatestByType(
		ctx, questID, []models.EventType{models.EventRunCompleted},
	)
	if err != nil {
		return nil, fmt.Errorf("finding run finishes: %w", err)
	}

	times, err := s.runEventRepo.TimesByType(ctx, questID, meaningfulTypes, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("finding event times: %w", err)
	}
	questPace := QuestBaseline(times)

	now := time.Now()
	summaries := make(map[string]RunSummary, len(runs))
	// Every run of the quest, stream or no stream: progress is a state
	// question, and the runs recorded before there was a stream still have
	// completions to count. Their status is the one thing the stream must
	// answer, and a run without one has no pace to read.
	for _, run := range runs {
		_, finished := finishes[run.Code]
		progress := BuildRunProgress(
			inPlay,
			booleanSet(completed[run.Code]),
			RunActivity{Closed: booleanSet(closed[run.Code]), Finished: finished},
			nil,
		)
		status := StatusNotStarted
		if run.HasStarted {
			status = runCadenceStatus(times[run.Code], questPace, finished, now)
		}
		summaries[run.Code] = RunSummary{
			Done:   progress.Done,
			Total:  progress.Total,
			Status: status,
		}
	}
	return summaries, nil
}

func booleanSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// runCadenceStatus mirrors the run page's ladder: finished first, then the quiet against pace.
func runCadenceStatus(
	times []time.Time, questBaseline time.Duration, finished bool, now time.Time,
) TeamStatus {
	if finished {
		return StatusFinished
	}

	baseline := pacedMedian(gapsBetween(times))
	if baseline == 0 && questBaseline > 0 {
		baseline = max(questBaseline, cadenceMinBaseline)
	}
	last := lastOf(times)
	if baseline == 0 || last.IsZero() {
		return StatusStarted
	}

	quiet := now.Sub(last)
	if quiet < 0 {
		return StatusStarted
	}
	switch cadenceClass(quiet, baseline, true) {
	case CadencePlaying:
		return StatusPlaying
	case CadenceIdle:
		return StatusIdle
	case CadenceStalled:
		return StatusStalled
	case CadenceAbandoned:
		return StatusAbandoned
	}
	return StatusStarted
}
