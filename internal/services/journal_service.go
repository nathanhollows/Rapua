package services

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// JournalEntryKind names what one line of the diary is.
type JournalEntryKind string

const (
	// JournalFind is an objective whose proof context cleared.
	JournalFind JournalEntryKind = "find"
	// JournalMiss is a submission that did not complete its block. It is never
	// called wrong: the log records completion and worth, not correctness, and
	// only the block knows what right means.
	JournalMiss JournalEntryKind = "miss"
	// JournalPhoto is a player upload.
	JournalPhoto JournalEntryKind = "photo"
	// JournalMessage is a message arriving from the organisers.
	JournalMessage JournalEntryKind = "message"
	// JournalMilestone is a section closing: a moment at its own time, never a
	// heading over the entries around it.
	JournalMilestone JournalEntryKind = "milestone"
	// JournalStart is the opening line, folding the first greeting and the
	// run's name into the one thing that happened.
	JournalStart JournalEntryKind = "start"
	// JournalFinish is the story's last line: the root objective completing.
	JournalFinish JournalEntryKind = "finish"
)

// JournalEntry is one line of the run's diary.
type JournalEntry struct {
	Kind JournalEntryKind
	Time time.Time
	// Title is the objective the entry is about, or the run's name for the
	// opening line. Empty where the entry names itself, like a photo.
	Title string
	// Slug reopens the objective's own page, which is where a reveal's content
	// survives. Empty when the objective no longer resolves.
	Slug string
	// Points is the credit that came with the moment, as ledgered: for a find,
	// everything the objective had paid by the time it cleared.
	Points int

	// MediaURL and MediaType belong to photo entries. The URL is the one the
	// upload event recorded, never re-fetched.
	MediaURL  string
	MediaType string

	// Message is the text of a message entry, and Read and ReadWait its
	// dismissal record: Read is whether it was cleared, ReadWait how long it
	// waited to be.
	Message  string
	Read     bool
	ReadWait time.Duration

	// Found and Children belong to milestone entries: how many of a section's
	// children had landed when it closed.
	Found    int
	Children int
}

// JournalView is what the journal page renders, plus the numbers the chrome
// carries.
type JournalView struct {
	Entries []JournalEntry
	Done    int
	Total   int
}

// GetJournal builds the player's diary and the numbers the chrome carries. The
// entries come from the run's own history; the progress comes from the tree as
// it stands, because the bar is about now and the diary is about then.
func (s *NavigationService) GetJournal(ctx context.Context, team *models.Run) (JournalView, error) {
	events, err := s.runEventRepo.ListByRun(ctx, team.Code)
	if err != nil {
		return JournalView{}, fmt.Errorf("loading run events: %w", err)
	}

	objectives, _, complete, err := s.loader.load(ctx, team)
	if err != nil {
		return JournalView{}, err
	}

	view := BuildJournal(events, objectives, complete)
	progress := navigation.ComputeProgress(objectives, complete)
	view.Done = progress.Done
	view.Total = progress.Total()
	return view, nil
}

// BuildJournal turns a run's recorded events into the player's diary.
//
// Curation is the job: most of what the stream holds is not an entry. Views and
// listings are cadence data; a submission that completed its block is the
// moment a find already tells; reads annotate their message rather than
// becoming lines of their own.
func BuildJournal(
	events []models.RunEvent, objectives []models.Objective, complete map[string]bool,
) JournalView {
	byID := make(map[string]models.Objective, len(objectives))
	for _, obj := range objectives {
		byID[obj.ID] = obj
	}

	reads := notificationReads(events)
	children := inPlayChildren(objectives)
	credits := objectiveCredits(events)

	entries := make([]JournalEntry, 0, len(events))
	for i, event := range events {
		if entry, ok := journalEntryFor(event, credits[i], byID, reads, children, complete); ok {
			entries = append(entries, entry)
		}
	}

	// The opening line takes the run's name as last set, wherever in the kept
	// stream that was: the name is how the run is remembered, not a mid-run
	// event worth its own line.
	for i := range entries {
		if entries[i].Kind == JournalStart {
			entries[i].Title = lastName(events)
			break
		}
	}

	slices.Reverse(entries)
	return JournalView{Entries: entries}
}

// journalEntryFor turns one event into a diary line. The switch is the whole
// curation rule: what it does not name, the page does not show.
func journalEntryFor(
	event models.RunEvent,
	objectivePaid int,
	byID map[string]models.Objective,
	reads map[string]time.Time,
	children map[string][]string,
	complete map[string]bool,
) (JournalEntry, bool) {
	switch event.Type {
	case models.EventRunStarted:
		return JournalEntry{Kind: JournalStart, Time: event.CreatedAt}, true

	case models.EventRunCompleted:
		return JournalEntry{Kind: JournalFinish, Time: event.CreatedAt}, true

	case models.EventObjectiveContextCompleted:
		return journalCompletion(event, objectivePaid, byID)

	case models.EventBlockAnswered:
		return journalAnswer(event, byID)

	case models.EventUploadAdded:
		return journalUpload(event)

	case models.EventNotificationSent:
		return journalMessage(event, reads)

	case models.EventSectionFinished:
		return journalSection(event, byID, children, complete)

	case models.EventRunNamed, models.EventObjectivesListed, models.EventObjectiveViewed,
		models.EventNotificationRead:
		// Recorded, not shown. The name folds into the opening line; views and
		// listings are cadence data; reads annotate their own message.
		return JournalEntry{}, false
	}
	return JournalEntry{}, false
}

// journalCompletion is a find: the proof context clearing. A reveal completing
// is not an entry, because the content is on the page at the moment it
// completes, so telling the player about it says what they just did. The find's
// link is where the reward survives for later.
//
// The points are what the objective had paid by the time it cleared, so a find
// badges with what it gave rather than with the closing submission's credit.
func journalCompletion(
	event models.RunEvent, objectivePaid int, byID map[string]models.Objective,
) (JournalEntry, bool) {
	var payload models.ObjectiveContextCompletedData
	if json.Unmarshal(event.Data, &payload) != nil {
		return JournalEntry{}, false
	}
	if payload.Context != game.ContextObjectiveProof {
		return JournalEntry{}, false
	}
	entry := JournalEntry{
		Kind:   JournalFind,
		Time:   event.CreatedAt,
		Title:  "An objective",
		Points: objectivePaid,
	}
	if obj, ok := byID[event.ObjectiveID]; ok {
		entry.Title = obj.Title
		entry.Slug = obj.Slug
	}
	return entry, true
}

// journalAnswer is a miss, and only a miss: a submission that completed its
// block is the moment a find already tells.
func journalAnswer(
	event models.RunEvent, byID map[string]models.Objective,
) (JournalEntry, bool) {
	var payload models.BlockAnsweredData
	if json.Unmarshal(event.Data, &payload) != nil || payload.Completed {
		return JournalEntry{}, false
	}
	entry := JournalEntry{Kind: JournalMiss, Time: event.CreatedAt}
	if event.ObjectiveID != "" {
		entry.Title = "An objective"
		if obj, ok := byID[event.ObjectiveID]; ok {
			entry.Title = obj.Title
		}
	}
	return entry, true
}

func journalUpload(event models.RunEvent) (JournalEntry, bool) {
	var payload models.UploadAddedData
	if json.Unmarshal(event.Data, &payload) != nil {
		return JournalEntry{}, false
	}
	return JournalEntry{
		Kind:      JournalPhoto,
		Time:      event.CreatedAt,
		MediaURL:  payload.URL,
		MediaType: payload.MediaType,
	}, true
}

func journalMessage(event models.RunEvent, reads map[string]time.Time) (JournalEntry, bool) {
	var payload models.NotificationData
	if json.Unmarshal(event.Data, &payload) != nil {
		return JournalEntry{}, false
	}
	entry := JournalEntry{
		Kind:    JournalMessage,
		Time:    event.CreatedAt,
		Message: payload.Content,
	}
	if readAt, ok := reads[payload.NotificationID]; ok {
		entry.Read = true
		entry.ReadWait = readAt.Sub(event.CreatedAt)
	}
	return entry, true
}

// journalSection is a milestone: the close, when it happened, and what it
// closed with. A section the tree no longer holds cannot be named, so it is
// the one event the feed drops outright.
func journalSection(
	event models.RunEvent,
	byID map[string]models.Objective,
	children map[string][]string,
	complete map[string]bool,
) (JournalEntry, bool) {
	obj, ok := byID[event.ObjectiveID]
	if !ok {
		return JournalEntry{}, false
	}
	kids := children[event.ObjectiveID]
	found := 0
	for _, child := range kids {
		if complete[child] {
			found++
		}
	}
	return JournalEntry{
		Kind:     JournalMilestone,
		Time:     event.CreatedAt,
		Title:    obj.Title,
		Slug:     obj.Slug,
		Found:    found,
		Children: len(kids),
	}, true
}

// objectiveCredits maps each context completion, by its index in the
// stream, to what the objective had paid by then: the ledger's own answers for
// it, summed. The completion payload carries only the closing submission's
// credit, which understates an objective whose earlier blocks paid and reads as
// nothing when the block that closed it carried no points.
func objectiveCredits(events []models.RunEvent) map[int]int {
	paid := make(map[string]int)
	credits := make(map[int]int)
	for i, event := range events {
		if event.Type == models.EventBlockAnswered && event.ObjectiveID != "" {
			paid[event.ObjectiveID] += event.Points
		}
		if event.Type == models.EventObjectiveContextCompleted {
			credits[i] = paid[event.ObjectiveID]
		}
	}
	return credits
}

// notificationReads maps each message to the first time the run cleared it.
func notificationReads(events []models.RunEvent) map[string]time.Time {
	reads := make(map[string]time.Time)
	for _, event := range events {
		if event.Type != models.EventNotificationRead {
			continue
		}
		var payload models.NotificationData
		if json.Unmarshal(event.Data, &payload) != nil {
			continue
		}
		if _, seen := reads[payload.NotificationID]; !seen {
			reads[payload.NotificationID] = event.CreatedAt
		}
	}
	return reads
}

// inPlayChildren maps each objective to its in-play children. Drafted nodes are
// not work anyone still owes, so a milestone counts without them.
func inPlayChildren(objectives []models.Objective) map[string][]string {
	inPlay := make(map[string]bool, len(objectives))
	for _, obj := range navigation.InPlay(objectives) {
		inPlay[obj.ID] = true
	}

	children := make(map[string][]string, len(objectives))
	for _, obj := range objectives {
		if obj.ParentID == "" || !inPlay[obj.ID] || !inPlay[obj.ParentID] {
			continue
		}
		children[obj.ParentID] = append(children[obj.ParentID], obj.ID)
	}
	return children
}

// lastName is the run's name as last set in the stream.
func lastName(events []models.RunEvent) string {
	name := ""
	for _, event := range events {
		if event.Type != models.EventRunNamed {
			continue
		}
		var payload models.RunNamedData
		if json.Unmarshal(event.Data, &payload) == nil {
			name = payload.Name
		}
	}
	return name
}
