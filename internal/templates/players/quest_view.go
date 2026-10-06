package templates

import (
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// QuestView is the frontier regrouped under the sections its rows came from.
// The frontier omits an open section and lists its children in its place, which
// suits the engine but makes three parts of one task read as three tasks.
type QuestView struct {
	Groups []QuestGroup
}

// QuestGroup is one section and the rows of it the run can work on, or a single
// top-level row standing on its own.
type QuestGroup struct {
	// IsSection is false for a row under the root: wrapping it would invent a
	// container the author never wrote.
	IsSection bool
	ID        string
	Slug      string
	Title     string
	// Rule is how this section offers its children, in a player's words. Empty
	// on a standalone row.
	Rule string
	Rows []QuestRow
}

// QuestRow is one objective on offer.
type QuestRow struct {
	ID    string
	Slug  string
	Title string
	// IsDoor marks an objective with no proof: there is nothing to do but open
	// it, so the row must not promise work.
	IsDoor bool
	// CanFinish marks a section whose band is met with room still above it.
	CanFinish   bool
	FinishLabel string
	// IconSVG is the icon of the first proof block that asks for something.
	// Empty when none does, and the row falls back to its own marker.
	IconSVG string
}

// routingRule words a section's routing for players, who are never told
// "randomised".
func routingRule(parent models.Objective, offered int) string {
	switch parent.Routing {
	case models.RouteStrategyOrdered:
		return "Next up"
	case models.RouteStrategyRandomised:
		if parent.MaxNext > 0 {
			return pluralise(parent.MaxNext, "picked for your team")
		}
		return "Picked for your team"
	case models.RouteStrategyFreeRoam:
		// One row has no order to be free of.
		if offered > 1 {
			return "In any order"
		}
		return ""
	default:
		return ""
	}
}

func pluralise(n int, suffix string) string {
	words := []string{"None", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine"}
	if n < len(words) {
		return words[n] + " " + suffix
	}
	return "Several " + suffix
}

// finishLabel falls back to "Finish": the band decides the button appears, so a
// blank label would render an empty button.
func finishLabel(objective models.Objective) string {
	if objective.FinishLabel != "" {
		return objective.FinishLabel
	}
	return "Finish"
}

// BuildQuestView groups the frontier under the sections its rows came from.
//
// It needs the whole tree because a frontier row carries only its parent's ID.
// A row whose parent is missing is kept standing alone: dropping it would hide
// work the run can do behind a lookup failure.
//
// A nil HasProof reads as "no proof", which makes rows doors: that opens,
// where the other way round would promise work that is not there.
func BuildQuestView(view *services.PlayerObjectiveView) QuestView {
	frontier := view.Frontier
	byID := make(map[string]models.Objective, len(view.Objectives))
	for _, objective := range view.Objectives {
		byID[objective.ID] = objective
	}

	var built QuestView
	// Groups keep the frontier's tree order: sorting would rearrange a quest
	// the author laid out by hand.
	index := make(map[string]int, len(view.Objectives))

	for _, objective := range frontier.Available {
		row := QuestRow{
			ID:      objective.ID,
			Slug:    objective.Slug,
			Title:   objective.Title,
			IsDoor:  !view.HasProof[objective.ID],
			IconSVG: blocks.IconSVGForType(view.FirstProofBlock[objective.ID]),
		}
		if frontier.StatusOf(objective.ID) == navigation.StatusFinishable {
			row.CanFinish = true
			row.FinishLabel = finishLabel(objective)
			// The finish button is the whole task, not a door into one.
			row.IsDoor = false
		}

		parent, hasParent := byID[objective.ParentID]
		if !hasParent || parent.ParentID == "" {
			// Under the root or orphaned: no section a player would recognise.
			built.Groups = append(built.Groups, QuestGroup{
				ID: objective.ID, Slug: objective.Slug, Rows: []QuestRow{row},
			})
			continue
		}

		at, seen := index[parent.ID]
		if !seen {
			at = len(built.Groups)
			index[parent.ID] = at
			built.Groups = append(built.Groups, QuestGroup{
				IsSection: true,
				ID:        parent.ID,
				Slug:      parent.Slug,
				Title:     parent.Title,
			})
		}
		built.Groups[at].Rows = append(built.Groups[at].Rows, row)
	}

	// Set last because the rule depends on the row count.
	for i := range built.Groups {
		if !built.Groups[i].IsSection {
			continue
		}
		built.Groups[i].Rule = routingRule(byID[built.Groups[i].ID], len(built.Groups[i].Rows))
	}
	return built
}
