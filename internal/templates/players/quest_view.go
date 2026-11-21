package templates

import (
	"fmt"
	"strconv"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// QuestView puts back the sections the frontier omits, as headings rather than
// boxes: a container is not something a player can do.
type QuestView struct {
	Places []QuestPlace
}

// QuestPlace is flat, not recursive: nested cards indent every level equally and
// squeeze the tasks into the narrowest column. Depth lives in Path instead.
type QuestPlace struct {
	// IsSection is false for rows directly under the root.
	IsSection bool
	ID        string
	Slug      string
	Title     string
	Rule      string
	// Path names the drawn sections above this one, outermost first.
	Path []string
	Rows []QuestRow

	// Finishes live on the place because a row would read as a task named like the
	// heading. Several when hidden sections finish under a drawn ancestor.
	Finishes []QuestFinish

	// Found, Total and Minimum are the band as it stands, and zero on a place
	// that is not a section.
	Found   int
	Total   int
	Minimum int

	Pill    string
	Ordered bool
	Hue     int

	// NamedByGroup suppresses the title the group heading already shows.
	NamedByGroup bool
}

// QuestFinish carries its own title because its place may be an ancestor's.
type QuestFinish struct {
	// Slug is the finished section's own, not its place's.
	Slug  string
	Label string
	Title string
	// WholeQuest is true for the root, where the dialog ends the run.
	WholeQuest bool
}

// QuestGroup holds places under one outermost section. Anything deeper stays in
// a place's Path, so the screen reads group, section, task and no further.
type QuestGroup struct {
	// Title is empty for places with no section above them.
	Title  string
	Places []QuestPlace
}

// Groups merges only neighbours: pulling two apart would rearrange the author's
// order.
func (q QuestView) Groups() []QuestGroup {
	var groups []QuestGroup
	for i, place := range q.Places {
		title := ""
		if len(place.Path) > 0 {
			title = place.Path[0]
			place.Path = place.Path[1:]
		} else if place.IsSection && i+1 < len(q.Places) {
			// Joins its own subsections' group so the title prints once.
			if next := q.Places[i+1]; len(next.Path) > 0 && next.Path[0] == place.Title {
				title = place.Title
				place.NamedByGroup = true
			}
		}
		if n := len(groups); n > 0 && groups[n-1].Title == title && title != "" {
			groups[n-1].Places = append(groups[n-1].Places, place)
			continue
		}
		groups = append(groups, QuestGroup{Title: title, Places: []QuestPlace{place}})
	}
	return groups
}

// HueOfObjective keeps a task's list colour when it is opened. Zero when no drawn
// section contains it.
func HueOfObjective(objectives []models.Objective, objective models.Objective) int {
	byID := make(map[string]models.Objective, len(objectives))
	for _, o := range objectives {
		byID[o.ID] = o
	}
	place, _ := navigation.PlaceFor(byID, objective)
	if place.ID == "" {
		return 0
	}
	return sectionHues(objectives)[place.ID]
}

// Counted over the whole authored tree so opening one section never recolours
// another. Undrawn sections (pass-through, past the depth cap) take no number.
func sectionHues(objectives []models.Objective) map[string]int {
	byID := make(map[string]models.Objective, len(objectives))
	hasChildren := make(map[string]bool, len(objectives))
	for _, objective := range objectives {
		byID[objective.ID] = objective
		if objective.ParentID != "" {
			hasChildren[objective.ParentID] = true
		}
	}
	hues := make(map[string]int, len(objectives))
	next := 0
	for _, objective := range objectives {
		if objective.ParentID == "" || !hasChildren[objective.ID] {
			continue
		}
		if navigation.KindOf(byID, objective) != navigation.SectionDrawn {
			continue
		}
		hues[objective.ID] = HueFor(next)
		next++
	}
	return hues
}

type QuestRow struct {
	ID    string
	Slug  string
	Title string
	// Description is read standing in front of the thing, where the title is read
	// in a list.
	Description string
	// IsDoor is an objective with no proof, so the row must not promise work.
	IsDoor bool
	// IconSVG is empty when the proof asks nothing.
	IconSVG string
}

// RoutingRule is exported so the editor's preview words the rule exactly as the
// game does. Players are never told "randomised", only what is theirs.
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

var numberWords = []string{"none", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}

// numberWord falls back to the numeral past nine, as "Complete all several" is
// not a rule a player can follow.
func numberWord(n int) string {
	if n >= 0 && n < len(numberWords) {
		return numberWords[n]
	}
	return strconv.Itoa(n)
}

func pluralise(n int, suffix string) string {
	return numberWord(n) + " " + suffix
}

// A ceiling below the child count is a pick: "Complete all three" would tell the
// player to keep going after the section had shut.
func bandWords(band game.Band, childCount int) string {
	switch {
	case band.AutoCompletes() && band.Max < childCount:
		return "Pick " + numberWord(band.Min)
	case band.AutoCompletes():
		if childCount > 1 {
			return "Complete all " + numberWord(childCount)
		}
		return ""
	case band.Min <= 0:
		return "Finish whenever you are ready"
	default:
		return "Complete at least " + numberWord(band.Min)
	}
}

// A range means the section can close before everything is found, so the rule
// says the least. The band counts children in play; routing counts what is on
// offer.
func questRule(objective models.Objective, childCount, offered int) string {
	bandText := bandWords(game.FillBand(objective.ChildrenMin, objective.ChildrenMax, childCount), childCount)

	routing := RoutingRule(objective, offered)
	switch {
	case bandText == "":
		return routing
	case routing == "":
		return bandText
	default:
		return bandText + " · " + routing
	}
}

// The count towards a band beats a rule read once. Free roam is never said: only
// what can be done is on offer, so any order is the only order.
func questPill(objective models.Objective, band game.Band, childCount, found int) string {
	if objective.Routing == models.RouteStrategyOrdered {
		return "Next up"
	}
	if band.Min > 1 {
		return fmt.Sprintf("%d of %d done", min(found, band.Min), band.Min)
	}
	text := bandWords(band, childCount)
	if objective.Routing == models.RouteStrategyRandomised && text == "" {
		return "Picked for your team"
	}
	return text
}

// SectionPill is the heading pill a player meets before doing anything in a
// section, so an editor can show a row in the words it will really have.
func SectionPill(section models.Objective, childCount int) string {
	band := game.FillBand(section.ChildrenMin, section.ChildrenMax, childCount)
	return questPill(section, band, childCount, 0)
}

// The band decides the button appears, so a blank label must not render an
// empty one.
func finishLabel(objective models.Objective) string {
	if objective.FinishLabel != "" {
		return objective.FinishLabel
	}
	return "Finish"
}

// BuildQuestView needs the quest's whole tree because a frontier row carries only
// its parent's id. A row whose parent is missing is kept and shown alone, as
// dropping it would hide work behind a lookup failure.
//
// A nil HasProof reads as no proof: a door opens, where the reverse would
// promise work that is not there.
func BuildQuestView(view *services.PlayerObjectiveView) QuestView {
	byID := make(map[string]models.Objective, len(view.Objectives))
	for _, objective := range view.Objectives {
		byID[objective.ID] = objective
	}

	b := builder{byID: byID, view: view}
	hues := sectionHues(view.Objectives)
	var built QuestView
	// Sorting would rearrange a quest the author laid out by hand.
	index := make(map[string]int, len(view.Objectives))
	placeAt := func(place models.Objective, path []string) int {
		key := place.ID
		if at, seen := index[key]; seen {
			return at
		}
		index[key] = len(built.Places)
		built.Places = append(built.Places, QuestPlace{
			IsSection: key != "",
			ID:        place.ID,
			Slug:      place.Slug,
			Title:     place.Title,
			Path:      path,
		})
		return len(built.Places) - 1
	}

	for _, objective := range view.Frontier.Available {
		// A finishable section is a press on its own heading, not a task: a row
		// would print the name twice.
		if view.Frontier.StatusOf(objective.ID) == navigation.StatusFinishable {
			place, path := b.finishPlace(objective)
			at := placeAt(place, path)
			built.Places[at].Finishes = append(built.Places[at].Finishes, QuestFinish{
				Slug:       objective.Slug,
				Label:      finishLabel(objective),
				Title:      objective.Title,
				WholeQuest: objective.ParentID == "",
			})
			continue
		}

		place, path := b.placeFor(objective)
		at := placeAt(place, path)
		built.Places[at].Rows = append(built.Places[at].Rows, b.rowFor(objective))
	}

	// Last, because the counts read what the place ended up holding.
	inPlay := inPlaySet(view.Objectives)
	for i := range built.Places {
		if !built.Places[i].IsSection {
			continue
		}
		objective := byID[built.Places[i].ID]
		total, found := 0, 0
		for _, child := range view.Objectives {
			if child.ParentID != objective.ID || !inPlay[child.ID] {
				continue
			}
			total++
			if view.Frontier.StatusOf(child.ID) == navigation.StatusComplete {
				found++
			}
		}
		built.Places[i].Found = found
		built.Places[i].Total = total
		built.Places[i].Minimum = game.FillBand(objective.ChildrenMin, objective.ChildrenMax, total).Min
		built.Places[i].Rule = questRule(objective, total, len(built.Places[i].Rows))
		band := game.FillBand(objective.ChildrenMin, objective.ChildrenMax, total)
		built.Places[i].Pill = questPill(objective, band, total, found)
		built.Places[i].Ordered = objective.Routing == models.RouteStrategyOrdered
		built.Places[i].Hue = hues[objective.ID]
	}
	return built
}

func inPlaySet(objectives []models.Objective) map[string]bool {
	set := make(map[string]bool, len(objectives))
	for _, objective := range navigation.InPlay(objectives) {
		set[objective.ID] = true
	}
	return set
}

type builder struct {
	byID map[string]models.Objective
	view *services.PlayerObjectiveView
}

func (b builder) placeFor(objective models.Objective) (models.Objective, []string) {
	place, path := navigation.PlaceFor(b.byID, objective)
	return place, titlesOf(path)
}

func (b builder) finishPlace(section models.Objective) (models.Objective, []string) {
	place, path := navigation.FinishPlaceFor(b.byID, section)
	return place, titlesOf(path)
}

func titlesOf(sections []models.Objective) []string {
	titles := make([]string, 0, len(sections))
	for _, section := range sections {
		titles = append(titles, section.Title)
	}
	return titles
}

func (b builder) rowFor(objective models.Objective) QuestRow {
	return QuestRow{
		ID:          objective.ID,
		Slug:        objective.Slug,
		Title:       objective.Title,
		Description: objective.Description,
		IsDoor:      !b.view.HasProof[objective.ID],
		IconSVG:     blocks.IconSVGForType(b.view.FirstProofBlock[objective.ID]),
	}
}
