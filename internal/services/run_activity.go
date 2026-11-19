package services

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
)

type ActivityEntry struct {
	RunCode string
	RunName string
	RunLogEntry
}

// A proof's completion row is left out: the answer that cleared it already says
// so, and carries the points.
//
//nolint:gochecknoglobals // A definition, not state: one place says what the feed shows.
var feedKinds = []RunLogKind{RunLogStart, RunLogAnswer, RunLogSection, RunLogUpload, RunLogFinish}

// A wrong answer is left out: the feed is for what teams achieved, and a
// misread QR would drown it.
func BuildActivity(
	events []models.RunEvent, objectives []models.Objective, names map[string]string, limit int,
) []ActivityEntry {
	byID := make(map[string]models.Objective, len(objectives))
	for _, objective := range objectives {
		byID[objective.ID] = objective
	}
	blockNames := blockNamesByType()
	tries := make(map[string]int)

	feed := make([]ActivityEntry, 0, limit)
	for _, event := range events {
		entry, ok := runLogEntryFor(event, byID, blockNames, tries, 0)
		if !ok || !slices.Contains(feedKinds, entry.Kind) {
			continue
		}
		if entry.Kind == RunLogAnswer && !entry.Completed {
			continue
		}
		feed = append(feed, ActivityEntry{RunCode: event.RunCode, RunName: names[event.RunCode], RunLogEntry: entry})
	}

	slices.Reverse(feed)
	if len(feed) > limit {
		feed = feed[:limit]
	}
	return feed
}

func (s *RunService) RecentActivity(
	ctx context.Context, quest *models.Quest, within time.Duration, limit int,
) ([]ActivityEntry, error) {
	events, err := s.runEventRepo.ListByQuestSince(ctx, quest.ID, time.Now().Add(-within))
	if err != nil {
		return nil, fmt.Errorf("loading the quest's recent events: %w", err)
	}

	names := make(map[string]string, len(quest.Runs))
	for _, run := range quest.Runs {
		names[run.Code] = run.Name
	}
	return BuildActivity(events, quest.Objectives, names, limit), nil
}
