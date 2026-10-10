package templates

import (
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// QuestView is the frontier arranged as the places a run is standing in.
//
// The frontier omits an open section and lists its children in its place, which
// suits the engine and makes three parts of one task read as three tasks. This
// puts the section back, but as a heading rather than a box: a container is not
// something a player can do, and drawing it like one gives the screen two kinds
// of object when only one is real.
type QuestView struct {
	Places []QuestPlace
}

// QuestPlace is one section and the rows of it on offer, or the rows that
// belong to no section at all.
//
// Flat, not recursive. Nesting cards inside cards indents every level by the
// same amount, so a reader counts margins to work out where they are, and the
// tasks end up in the narrowest column on the screen. Depth is carried by Path
// instead, which costs no width at all.
type QuestPlace struct {
	// IsSection is false for rows sitting directly under the root, which belong
	// to no container a player would recognise.
	IsSection bool
	ID        string
	Slug      string
	Title     string
	// Rule is how this place offers what it is showing, in the words a player
	// reads.
	Rule string
	// Path names the drawn sections above this one, outermost first. It is what
	// replaces the nesting: where you came from, in small type, rather than a
	// border around everything you are looking at.
	Path []string
	Rows []QuestRow
}

// QuestRow is one objective on offer.
type QuestRow struct {
	ID    string
	Slug  string
	Title string
	// Description is the detail a player needs on arrival. The title is read
	// in a list; this is read standing in front of the thing.
	Description string
	// IsDoor marks an objective with no proof: there is nothing to do but open
	// it, so the row must not promise work.
	IsDoor bool
	// CanFinish marks a section whose band is met with room still above it.
	CanFinish   bool
	FinishLabel string
	// IconSVG is the first block in this objective's proof that asks for
	// something: the plainest answer to "what will this want from me". Empty
	// when the proof asks nothing.
	IconSVG string
}

// maxDrawnDepth is how many sections deep the player's screen ever draws.
//
// Past this a container passes through: it still routes, bands and gates, it
// simply has no place on the screen and its rows join the nearest section that
// does. The author's tree has no depth limit; the screen has a fixed one, so
// nesting for a band across four sections costs the player nothing.
const maxDrawnDepth = 2

// RoutingRule words a section's routing for players, who are never told
// "randomised": they are told three of these are theirs, which is the same fact
// and the better one.
//
// Exported because the objective editor previews a row inside its own section,
// and a preview wording the rule differently from the game would be describing
// something else.
func RoutingRule(parent models.Objective, offered int) string {
	switch parent.Routing {
	case models.RouteStrategyOrdered:
		return "next up"
	case models.RouteStrategyRandomised:
		if parent.MaxNext > 0 {
			return pluralise(parent.MaxNext, "picked for your team")
		}
		return "picked for your team"
	case models.RouteStrategyFreeRoam:
		// Said only where there is an order to be free of.
		if offered > 1 {
			return "in any order"
		}
		return ""
	default:
		return ""
	}
}

func pluralise(n int, suffix string) string {
	words := []string{"none", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	if n < len(words) {
		return words[n] + " " + suffix
	}
	return "several " + suffix
}

// finishLabel names the button on a finishable section. An author may name it;
// otherwise it says the plainest true thing, because the band is what decides
// the button appears and a blank label would render an empty one.
func finishLabel(objective models.Objective) string {
	if objective.FinishLabel != "" {
		return objective.FinishLabel
	}
	return "Finish"
}

// BuildQuestView groups the frontier into the places it is offering.
//
// objectives is the quest's whole tree, because a frontier row carries only its
// parent's id and a place needs the ancestors above it. A row whose parent is
// missing is kept and shown standing alone: losing it would hide work the run
// can do behind a lookup failure.
//
// A nil HasProof reads as "no proof", which makes rows doors: that opens, where
// the other way round would promise work that is not there.
func BuildQuestView(view *services.PlayerObjectiveView) QuestView {
	byID := make(map[string]models.Objective, len(view.Objectives))
	for _, objective := range view.Objectives {
		byID[objective.ID] = objective
	}

	b := builder{byID: byID, view: view}
	var built QuestView
	// Places keep the frontier's tree order: sorting would rearrange a quest
	// the author laid out by hand.
	index := make(map[string]int, len(view.Objectives))

	for _, objective := range view.Frontier.Available {
		place, path := b.placeFor(objective)
		key := place.ID
		at, seen := index[key]
		if !seen {
			at = len(built.Places)
			index[key] = at
			built.Places = append(built.Places, QuestPlace{
				IsSection: key != "",
				ID:        place.ID,
				Slug:      place.Slug,
				Title:     place.Title,
				Path:      path,
			})
		}
		built.Places[at].Rows = append(built.Places[at].Rows, b.rowFor(objective))
	}

	// Last, because the rule counts what the place ended up offering.
	for i := range built.Places {
		if !built.Places[i].IsSection {
			continue
		}
		built.Places[i].Rule = RoutingRule(byID[built.Places[i].ID], len(built.Places[i].Rows))
	}
	return built
}

type builder struct {
	byID map[string]models.Objective
	view *services.PlayerObjectiveView
}

// placeFor finds the section a row is shown under, and the drawn sections above
// it.
//
// It walks up to the root collecting ancestors, then keeps only the first
// maxDrawnDepth of them: the last is the place, the rest are the path, and
// anything below passes through. Walking from the root down rather than from
// the row up is what makes the cap count from the top, where an author thinks
// about it, rather than from wherever the row happens to sit.
func (b builder) placeFor(objective models.Objective) (models.Objective, []string) {
	var chain []models.Objective
	seen := map[string]bool{objective.ID: true}
	for id := objective.ParentID; id != ""; {
		parent, ok := b.byID[id]
		// A missing parent is an orphan and a repeat is a cycle. Both are
		// shapes storage can hold and lint reports; neither is a reason to
		// hide work the run can still do.
		if !ok || seen[parent.ID] {
			break
		}
		seen[parent.ID] = true
		// The root is the quest rather than a place in it.
		if parent.ParentID == "" {
			break
		}
		chain = append([]models.Objective{parent}, chain...)
		id = parent.ParentID
	}

	if len(chain) == 0 {
		return models.Objective{}, nil
	}
	if len(chain) > maxDrawnDepth {
		chain = chain[:maxDrawnDepth]
	}

	path := make([]string, 0, len(chain)-1)
	for _, ancestor := range chain[:len(chain)-1] {
		path = append(path, ancestor.Title)
	}
	return chain[len(chain)-1], path
}

func (b builder) rowFor(objective models.Objective) QuestRow {
	row := QuestRow{
		ID:          objective.ID,
		Slug:        objective.Slug,
		Title:       objective.Title,
		Description: objective.Description,
		IsDoor:      !b.view.HasProof[objective.ID],
		IconSVG:     blocks.IconSVGForType(b.view.FirstProofBlock[objective.ID]),
	}
	if b.view.Frontier.StatusOf(objective.ID) == navigation.StatusFinishable {
		row.CanFinish = true
		row.FinishLabel = finishLabel(objective)
		// The finish button is the whole task, not a door into one.
		row.IsDoor = false
	}
	return row
}
