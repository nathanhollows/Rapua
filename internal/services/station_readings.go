package services

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

const (
	// stationHereWindow is how recently a team must have opened a station for it
	// to count as there. A team that looked at it an hour ago has gone.
	stationHereWindow = 15 * time.Minute
	// stationQuietAfter is how long a team can go unseen before a facilitator is
	// told it may have stopped, so nobody waits on a team that has given up.
	stationQuietAfter = 20 * time.Minute
	// stationGiveUpAfter is how long unseen before a team is treated as gone, whatever
	// its history says. The game's own reading of abandonment needs a pace to compare
	// against, so a team with little history never earns it; nobody should wait
	// at a station for a team nobody has seen since yesterday.
	stationGiveUpAfter = 3 * time.Hour
)

// StationTeam is one team as a facilitator sees it.
type StationTeam struct {
	Code string
	Name string
	// LastSeen is the team's latest interaction, or when it began.
	LastSeen time.Time
	// Arrived is when the team opened the station, for a team that is there.
	Arrived time.Time
	// Quiet marks a team unseen for a while.
	Quiet bool
}

// StationReading answers, for one objective, who is still to come. Only teams
// that are playing are counted anywhere in it: a team that has finished,
// stopped or never started cannot make a facilitator wait.
type StationReading struct {
	Objective models.Objective
	Depth     int
	// Parent is the title of the section it sits in, empty at the top.
	Parent string
	// Section is an objective with others inside it.
	Section bool

	// ToCome can visit now and has not. Here has opened it lately. Been has
	// cleared it. Later cannot reach it yet but its route still includes it.
	ToCome, Here, Been, Later []StationTeam

	// What is left out, so a facilitator can see why a number is not larger.
	Finished, Stopped, Skipped int
}

// StationReport is every station of a quest read for one moment.
type StationReport struct {
	// Playing is teams that could still visit something. NotStarted holds codes
	// not yet used. Stopped is teams quiet long enough to be given up on.
	Playing, NotStarted, Stopped int
	Stations                     []StationReading
}

// stationRun is one team with everything a reading needs about it.
type stationRun struct {
	Run     models.Run
	Summary RunSummary
	// Progress is set for a playing team only.
	Progress RunProgress
	// Completed is the proof completion log, for a team that is not playing but
	// has still been somewhere.
	Completed map[string]bool
	// Viewed is when the team last opened each objective, lately.
	Viewed map[string]time.Time
	// Recent is the latest thing the team did of any kind, lately. A team looking
	// around has not answered anything, so its last answer undersells it.
	Recent time.Time
}

// lastSeen is the latest sign of the team: an answer, anything lately, or its start.
func (r stationRun) lastSeen() time.Time {
	seen := LastSeenOrStarted(r.Summary.LastSeen, r.Run)
	if r.Recent.After(seen) {
		return r.Recent
	}
	return seen
}

// status is the team's status as a facilitator should read it: one that is
// playing but has not been seen for long enough is not coming.
func (r stationRun) status(now time.Time) TeamStatus {
	if !r.Run.HasStarted || r.Summary.Status == StatusNotStarted {
		return StatusNotStarted
	}
	if playingStatus(r.Summary.Status) && now.Sub(r.lastSeen()) > stationGiveUpAfter {
		return StatusAbandoned
	}
	return r.Summary.Status
}

// playingStatus is whether a status is a team that could still turn up.
func playingStatus(status TeamStatus) bool {
	switch status {
	case StatusStarted, StatusPlaying, StatusIdle, StatusStalled:
		return true
	case StatusNotStarted, StatusAbandoned, StatusFinished:
		return false
	}
	return false
}

func teamLabel(run models.Run) string {
	if strings.TrimSpace(run.Name) != "" {
		return run.Name
	}
	return "Team " + run.Code
}

// buildStationReport reads every station against every team. It is pure so the
// rules can be tested without a quest: the loading is somewhere else.
func buildStationReport(inPlay []models.Objective, runs []stationRun, now time.Time) StationReport {
	report := StationReport{}
	titles := make(map[string]string, len(inPlay))
	roots := make(map[string]bool)
	children := childrenByParent(inPlay)
	for _, objective := range inPlay {
		titles[objective.ID] = objective.Title
		if objective.ParentID == "" {
			roots[objective.ID] = true
		}
	}
	depth := depthsByNode(inPlay)

	for _, run := range runs {
		switch status := run.status(now); {
		case status == StatusNotStarted:
			report.NotStarted++
		case status == StatusAbandoned:
			report.Stopped++
		case playingStatus(status):
			report.Playing++
		}
	}

	for _, objective := range inPlay {
		// The quest's own root is the game, not somewhere to stand.
		if objective.Secret || roots[objective.ID] {
			continue
		}
		reading := StationReading{
			Objective: objective,
			Depth:     max(0, depth[objective.ID]-1),
			Section:   len(children[objective.ID]) > 0,
		}
		if !roots[objective.ParentID] {
			reading.Parent = titles[objective.ParentID]
		}
		for _, run := range runs {
			placeOn(&reading, objective.ID, run, now)
		}
		sortStationTeams(&reading)
		report.Stations = append(report.Stations, reading)
	}
	return report
}

// placeOn files one team under a station.
func placeOn(r *StationReading, objectiveID string, run stationRun, now time.Time) {
	status := run.status(now)
	if status == StatusNotStarted {
		return
	}
	seen := run.lastSeen()
	team := StationTeam{
		Code: run.Run.Code, Name: teamLabel(run.Run), LastSeen: seen,
		Quiet: !seen.IsZero() && now.Sub(seen) > stationQuietAfter,
	}

	if !playingStatus(status) {
		// Gone or finished: it still has been where it went, and is otherwise
		// only a footnote.
		switch {
		case run.Completed[objectiveID]:
			r.Been = append(r.Been, team)
		case status == StatusFinished:
			r.Finished++
		default:
			r.Stopped++
		}
		return
	}

	node, ok := progressNode(run.Progress, objectiveID)
	switch {
	case ok && node.Done:
		r.Been = append(r.Been, team)
	case ok && node.RuledOut:
		r.Skipped++
	case ok && node.Locked:
		r.Later = append(r.Later, team)
	default:
		if when, viewed := run.Viewed[objectiveID]; viewed && now.Sub(when) <= stationHereWindow {
			team.Arrived = when
			r.Here = append(r.Here, team)
			return
		}
		r.ToCome = append(r.ToCome, team)
	}
}

func progressNode(progress RunProgress, objectiveID string) (RunProgressNode, bool) {
	for _, node := range progress.Nodes {
		if node.Objective.ID == objectiveID {
			return node, true
		}
	}
	return RunProgressNode{}, false
}

// sortStationTeams puts the most recently seen first where it tells a facilitator
// who is closest to arriving, and the rest by name so a list holds still.
func sortStationTeams(r *StationReading) {
	byRecent := func(teams []StationTeam) {
		sort.SliceStable(teams, func(i, j int) bool { return teams[i].LastSeen.After(teams[j].LastSeen) })
	}
	byRecent(r.ToCome)
	byRecent(r.Later)
	sort.SliceStable(r.Here, func(i, j int) bool { return r.Here[i].Arrived.After(r.Here[j].Arrived) })
	sort.SliceStable(r.Been, func(i, j int) bool { return r.Been[i].Name < r.Been[j].Name })
}

// ReadStations reads every station of a quest as it stands. It costs a progress
// read per playing team, which is why it works from the same loader the player
// engine does: a second reading of the rules would drift from the game.
func (s *RunService) ReadStations(ctx context.Context, questID string, now time.Time) (StationReport, error) {
	runs, err := s.teamRepo.FindAll(ctx, questID)
	if err != nil {
		return StationReport{}, fmt.Errorf("finding runs: %w", err)
	}
	summaries, err := s.GetQuestRunSummaries(ctx, questID)
	if err != nil {
		return StationReport{}, err
	}
	tree, err := s.objectiveRepo.FindTreeByQuestID(ctx, questID)
	if err != nil {
		return StationReport{}, fmt.Errorf("finding objectives: %w", err)
	}
	inPlay := navigation.InPlay(tree)

	completed, err := s.objectiveContextCompletionRepo.FindCompletedByQuest(ctx, questID, game.ContextObjectiveProof)
	if err != nil {
		return StationReport{}, fmt.Errorf("finding completed objectives: %w", err)
	}
	events, err := s.runEventRepo.ListByQuestSince(ctx, questID, now.Add(-stationHereWindow))
	if err != nil {
		return StationReport{}, fmt.Errorf("finding recent views: %w", err)
	}
	viewed, recent := recentActivity(events)

	loaded := make([]stationRun, 0, len(runs))
	for _, run := range runs {
		summary := summaries[run.Code]
		entry := stationRun{
			Run: run, Summary: summary, Viewed: viewed[run.Code], Recent: recent[run.Code],
			Completed: booleanSet(completed[run.Code]),
		}
		if status := entry.status(now); playingStatus(status) {
			entry.Progress, err = s.GetRunProgress(ctx, questID, run.Code)
			if err != nil {
				return StationReport{}, fmt.Errorf("reading run %s: %w", run.Code, err)
			}
		}
		loaded = append(loaded, entry)
	}
	return buildStationReport(inPlay, loaded, now), nil
}

// recentActivity reads the lately-recorded events: when each team last opened each
// objective, and the latest thing each did of any kind.
func recentActivity(events []models.RunEvent) (map[string]map[string]time.Time, map[string]time.Time) {
	viewed := make(map[string]map[string]time.Time)
	recent := make(map[string]time.Time)
	for _, event := range events {
		if event.CreatedAt.After(recent[event.RunCode]) {
			recent[event.RunCode] = event.CreatedAt
		}
		if event.Type != models.EventObjectiveViewed || event.ObjectiveID == "" {
			continue
		}
		if viewed[event.RunCode] == nil {
			viewed[event.RunCode] = make(map[string]time.Time)
		}
		if event.CreatedAt.After(viewed[event.RunCode][event.ObjectiveID]) {
			viewed[event.RunCode][event.ObjectiveID] = event.CreatedAt
		}
	}
	return viewed, recent
}

// StationKey is a short, stable name for a station, eight characters instead of an
// objective's whole ID. A facilitator's choice of stations is kept in a cookie, and
// a few thousand bytes of full IDs overflows one without any error: the browser
// just drops it. A key is derived from the ID, so it survives the quest being
// edited, and it only ever selects among the stations a link covers.
func StationKey(objectiveID string) string {
	sum := sha256.Sum256([]byte(objectiveID))
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:5]))
}
