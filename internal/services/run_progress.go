package services

import (
	"context"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// RunProgress is the in-play tree in order, with each node marked by what the run did.
type RunProgress struct {
	Nodes []RunProgressNode
	// Done and Total count leaves only, ruled-out ones excluded. A whole-tree
	// count would leave a run that finished over open branches part-done forever.
	Done     int
	Total    int
	Finished bool
}

// RunProgressNode is one row of the quest map.
type RunProgressNode struct {
	Objective models.Objective
	Depth     int
	// Section means routing applies; on a leaf it means nothing.
	Section bool
	// Done is a cleared proof, or a section closed by its children or by a press.
	Done bool
	// Started needs the stream: nothing else records a view.
	Started bool
	// Locked is the frontier's answer, so gates, depends and routing all count.
	Locked bool
	// RuledOut is a cut branch's leftovers, or anything open when the run finished.
	RuledOut bool
	// Found and Children count a section's live children; both zero on a leaf.
	Found    int
	Children int
}

// GetRunProgress builds the admin's read of one run's progress.
func (s *RunService) GetRunProgress(ctx context.Context, questID, runCode string) (RunProgress, error) {
	objectives, state, complete, err := s.loader.load(ctx, &models.Run{Code: runCode, QuestID: questID})
	if err != nil {
		return RunProgress{}, err
	}

	// The frontier gates the player's screens too; a second reading of the rules would drift.
	frontier := navigation.ComputeFrontier(objectives, state, complete)
	locked := make(map[string]bool)
	for _, objective := range navigation.InPlay(objectives) {
		if frontier.StatusOf(objective.ID) == navigation.StatusLocked {
			locked[objective.ID] = true
		}
	}

	return BuildRunProgress(
		navigation.InPlay(objectives), state.ProofCompleted, s.runActivity(ctx, runCode), locked,
	), nil
}

// RunActivity is what the run has done, as the map needs it.
type RunActivity struct {
	Touched  map[string]bool
	Closed   map[string]bool
	Finished bool
}

func BuildRunProgress(
	inPlay []models.Objective, completed map[string]bool, activity RunActivity, locked map[string]bool,
) RunProgress {
	children := childrenByParent(inPlay)
	depth := depthsByNode(inPlay)
	done := leavesDone(inPlay, children, completed)
	started := leavesStarted(inPlay, children, activity.Touched)
	settleSections(inPlay, children, done, started, activity)
	ruledOut := ruleOut(inPlay, done, activity.Finished)
	return assembleProgress(inPlay, children, depth, done, started, ruledOut, locked, activity.Finished)
}

// runActivity reads what the run has done, from its stream.
func (s *RunService) runActivity(ctx context.Context, runCode string) RunActivity {
	events, err := s.runEventRepo.ListByRun(ctx, runCode)
	if err != nil {
		// The marks are decoration on the map, so degrade rather than fail the page.
		return RunActivity{}
	}

	activity := RunActivity{Touched: make(map[string]bool), Closed: make(map[string]bool)}
	for _, event := range events {
		if event.Type == models.EventObjectiveViewed || event.Type == models.EventBlockAnswered {
			if event.ObjectiveID != "" {
				activity.Touched[event.ObjectiveID] = true
			}
		}
		if event.Type == models.EventSectionFinished {
			activity.Closed[event.ObjectiveID] = true
		}
		if event.Type == models.EventRunCompleted {
			activity.Finished = true
		}
	}
	return activity
}

func childrenByParent(inPlay []models.Objective) map[string][]string {
	children := make(map[string][]string)
	for _, objective := range inPlay {
		if objective.ParentID != "" {
			children[objective.ParentID] = append(children[objective.ParentID], objective.ID)
		}
	}
	return children
}

// depthsByNode relies on the tree arriving parents first.
func depthsByNode(inPlay []models.Objective) map[string]int {
	depth := make(map[string]int, len(inPlay))
	for _, objective := range inPlay {
		if objective.ParentID != "" {
			depth[objective.ID] = depth[objective.ParentID] + 1
		}
	}
	return depth
}

func leavesDone(
	inPlay []models.Objective, children map[string][]string, completed map[string]bool,
) map[string]bool {
	done := make(map[string]bool, len(inPlay))
	for _, objective := range inPlay {
		if len(children[objective.ID]) == 0 && completed[objective.ID] {
			done[objective.ID] = true
		}
	}
	return done
}

func leavesStarted(
	inPlay []models.Objective, children map[string][]string, touched map[string]bool,
) map[string]bool {
	started := make(map[string]bool, len(inPlay))
	for _, objective := range inPlay {
		if len(children[objective.ID]) == 0 && touched[objective.ID] {
			started[objective.ID] = true
		}
	}
	return started
}

// settleSections walks in reverse so every child is settled before its parent
// (the tree arrives parents first). It mirrors the engine's band: a section is
// met at its maximum, or below that only by the player's press. The engine's
// own proof gate is not mirrored: a completed child is already its evidence.
// A finished run settles every section, or passed-over branches read open forever.
func settleSections(
	inPlay []models.Objective, children map[string][]string,
	done, started map[string]bool, activity RunActivity,
) {
	for i := len(inPlay) - 1; i >= 0; i-- {
		objective := inPlay[i]
		kids := children[objective.ID]
		if len(kids) == 0 {
			continue
		}
		found := 0
		anyStarted := activity.Touched[objective.ID]
		for _, kid := range kids {
			if done[kid] {
				found++
			}
			if started[kid] || done[kid] {
				anyStarted = true
			}
		}
		band := game.FillBand(objective.ChildrenMin, objective.ChildrenMax, len(kids))
		done[objective.ID] = activity.Finished ||
			activity.Closed[objective.ID] ||
			found >= band.Max
		started[objective.ID] = anyStarted
	}
}

// ruleOut walks parents first so an ancestor's answer is already in hand.
func ruleOut(inPlay []models.Objective, done map[string]bool, finished bool) map[string]bool {
	ruledOut := make(map[string]bool, len(inPlay))
	for _, objective := range inPlay {
		if done[objective.ID] {
			continue
		}
		if finished {
			ruledOut[objective.ID] = true
			continue
		}
		if objective.ParentID == "" {
			continue
		}
		if done[objective.ParentID] || ruledOut[objective.ParentID] {
			ruledOut[objective.ID] = true
		}
	}
	return ruledOut
}

func assembleProgress(
	inPlay []models.Objective, children map[string][]string,
	depth map[string]int, done, started, ruledOut, locked map[string]bool, finished bool,
) RunProgress {
	progress := RunProgress{Finished: finished, Nodes: make([]RunProgressNode, 0, len(inPlay))}
	for _, objective := range inPlay {
		kids := children[objective.ID]
		node := RunProgressNode{
			Objective: objective,
			Depth:     depth[objective.ID],
			Section:   len(kids) > 0,
			Done:      done[objective.ID],
			Started:   started[objective.ID],
			Locked:    locked[objective.ID],
			RuledOut:  ruledOut[objective.ID],
		}
		for _, kid := range kids {
			if ruledOut[kid] {
				continue
			}
			node.Children++
			if done[kid] {
				node.Found++
			}
		}
		// Judged on the tree, not the counted children: all-ruled-out is still a section.
		if !node.Section {
			if !node.RuledOut {
				progress.Total++
			}
			if node.Done {
				progress.Done++
			}
		}
		progress.Nodes = append(progress.Nodes, node)
	}
	return progress
}
