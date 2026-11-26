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
	// LastSeen is the run's most recent block interaction, the same reading the
	// activity page uses. Zero until the run has done anything.
	LastSeen time.Time
}

// LastSeenFrom lifts each run's last-seen out of its summary, keyed by run code,
// for the leaderboard's tiebreak. The summaries already carry it, so no caller
// needs a second query for it.
func LastSeenFrom(summaries map[string]RunSummary) map[string]time.Time {
	lastSeen := make(map[string]time.Time, len(summaries))
	for code, summary := range summaries {
		lastSeen[code] = summary.LastSeen
	}
	return lastSeen
}

// LastSeenOrStarted is when a run was last known to be about: its latest block
// interaction, else when it began. Both lists read it, so they cannot disagree.
func LastSeenOrStarted(lastSeen time.Time, run models.Run) time.Time {
	if lastSeen.IsZero() {
		return run.StartedAt
	}
	return lastSeen
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

	lastSeen, err := s.LastSeenByRun(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("finding last seen times: %w", err)
	}

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
			Done:     progress.Done,
			Total:    progress.Total,
			Status:   status,
			LastSeen: lastSeen[run.Code],
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
	return cadenceClass(quiet, baseline, true).Status()
}
