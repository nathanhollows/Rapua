package templates

import (
	"strings"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tree(rows ...models.Objective) []models.Objective { return rows }

func obj(id, parent, title string) models.Objective {
	return models.Objective{ID: id, Slug: id, Title: title, ParentID: parent}
}

// viewOf is the service's own output, which is what the grouping takes: the
// value the handler already holds, rather than four arguments pulled out of it.
func viewOf(available, objectives []models.Objective) *services.PlayerObjectiveView {
	status := map[string]navigation.Status{}
	for _, row := range available {
		status[row.ID] = navigation.StatusAvailable
	}
	return &services.PlayerObjectiveView{
		Frontier:   navigation.Frontier{Available: available, Status: status},
		Objectives: objectives,
	}
}

// The frontier is flat on purpose: a section that is merely open does not
// appear, because its children are listed in its place. The view has to put
// that section back, or three parts of one task read as three unrelated tasks.
func TestQuestView_RebuildsTheSectionTheFrontierFlattened(t *testing.T) {
	rows := tree(
		obj("root", "", "The Perfumer's Garden"),
		obj("top", "root", "Choose your top note"),
		obj("bergamot", "top", "Bergamot"),
		obj("pepper", "top", "Pink pepper"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[2], rows[3]}, rows))

	require.Len(t, view.Places, 1)
	place := view.Places[0]
	assert.Equal(t, "Choose your top note", place.Title)
	assert.Empty(t, place.Path, "nothing above it but the quest")
	require.Len(t, place.Rows, 2)
	assert.Equal(t, "Bergamot", place.Rows[0].Title)
}

// A section inside a section is the path, not a box: the ancestor is named in
// small type above the heading and costs no indent at all.
func TestQuestView_AncestorsBecomeThePath(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("scent", "root", "Make your scent"),
		obj("top", "scent", "Choose your top note"),
		obj("bergamot", "top", "Bergamot"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[3]}, rows))

	require.Len(t, view.Places, 1)
	assert.Equal(t, "Choose your top note", view.Places[0].Title)
	assert.Equal(t, []string{"Make your scent"}, view.Places[0].Path)
}

// Past the drawn depth a container passes through: it still bands and routes,
// it simply has no place on the screen, and its rows join the last section that
// does. The author's tree has no depth limit; the screen has a fixed one.
func TestQuestView_DeeperSectionsPassThrough(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("one", "root", "Level one"),
		obj("two", "one", "Level two"),
		obj("three", "two", "Level three"),
		obj("leaf", "three", "A task"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[4]}, rows))

	require.Len(t, view.Places, 1)
	assert.Equal(t, "Level two", view.Places[0].Title, "the deepest drawn section")
	assert.Equal(t, []string{"Level one"}, view.Places[0].Path)
	require.Len(t, view.Places[0].Rows, 1)
	assert.Equal(t, "A task", view.Places[0].Rows[0].Title, "and the row is still offered")
}

// Rows from two passed-through sections meet in the place above them, which is
// the whole point: the screen shows where the player is, not every box the
// author drew around it.
func TestQuestView_PassedThroughRowsMerge(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("one", "root", "Level one"), obj("two", "one", "Level two"),
		obj("a", "two", "Deep A"), obj("leafa", "a", "Task A"),
		obj("b", "two", "Deep B"), obj("leafb", "b", "Task B"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[4], rows[6]}, rows))

	require.Len(t, view.Places, 1)
	assert.Len(t, view.Places[0].Rows, 2)
}

// A row under the root belongs to no section a player would recognise, and
// wrapping it in one would invent a container the author never wrote.
func TestQuestView_RowsUnderTheRootStandAlone(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("intro", "root", "Find out how to play"),
		obj("sect", "root", "A section"), obj("inner", "sect", "Inside"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[1], rows[3]}, rows))

	require.Len(t, view.Places, 2)
	assert.False(t, view.Places[0].IsSection)
	assert.Equal(t, "Find out how to play", view.Places[0].Rows[0].Title)
	assert.True(t, view.Places[1].IsSection)
}

// Places keep the frontier's order, which is the tree order the engine already
// walked: re-sorting would rearrange a quest the author laid out by hand.
func TestQuestView_KeepsTheFrontierOrder(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("a", "root", "Section A"), obj("a1", "a", "A one"),
		obj("b", "root", "Section B"), obj("b1", "b", "B one"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[4], rows[2]}, rows))

	require.Len(t, view.Places, 2)
	assert.Equal(t, "Section B", view.Places[0].Title)
	assert.Equal(t, "Section A", view.Places[1].Title)
}

// An objective with no proof is a door: opening it is the whole of what there
// is to do, so the row must not promise work.
func TestQuestView_RowWithoutProofIsADoor(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("read", "root", "Read the safety notice"))

	assert.True(t, BuildQuestView(viewOf([]models.Objective{rows[1]}, rows)).Places[0].Rows[0].IsDoor)

	withProof := viewOf([]models.Objective{rows[1]}, rows)
	withProof.HasProof = map[string]bool{"read": true}
	assert.False(t, BuildQuestView(withProof).Places[0].Rows[0].IsDoor)
}

// The row's icon is the first block in its proof that asks for something, so a
// card says what is coming without anyone classifying objectives into kinds.
func TestQuestView_RowBorrowsItsProofBlocksIcon(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("gate", "root", "Scan the tag"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.FirstProofBlock = map[string]string{"gate": "scan"}

	icon := BuildQuestView(view).Places[0].Rows[0].IconSVG
	assert.Contains(t, icon, "scan-qr-code")
}

func intPtr(n int) *int { return &n }

// A finishable section is the finish press on the section itself, not a row of
// its own: the heading already names it, and a row would print the name a
// second time.
func TestQuestView_FinishableSectionCarriesThePressOnItsPlace(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("wing", "root", "Visit the east wing"),
		obj("statue", "wing", "Find the statue"),
	)
	rows[1].FinishLabel = "Leave the wing"
	rows[1].ChildrenMin = intPtr(1)
	rows[1].ChildrenMax = intPtr(3)

	view := viewOf([]models.Objective{rows[1], rows[2]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	quest := BuildQuestView(view)

	require.Len(t, quest.Places, 1)
	place := quest.Places[0]
	assert.Equal(t, "Visit the east wing", place.Title)
	assert.True(t, place.CanFinish)
	assert.Equal(t, "Leave the wing", place.FinishLabel)
	assert.Equal(t, "wing", place.FinishSlug)
	require.Len(t, place.Rows, 1, "its children stay tasks; the section itself is not one")
	assert.Equal(t, "Find the statue", place.Rows[0].Title)
	assert.Equal(t, 0, place.Found)
	assert.Equal(t, 1, place.Total)
	assert.Equal(t, "Complete at least one", place.Rule, "a ranged band says what the least is")
}

// An author who names no label still gets a button: the band decides one
// appears, and a blank label would render an empty one.
func TestQuestView_FinishLabelFallsBackToPlainWords(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("wing", "root", "Visit the east wing"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	assert.Equal(t, "Finish", BuildQuestView(view).Places[0].FinishLabel)
}

// A section states its band and its progress, press or no press.
func TestQuestView_SectionStatesItsBandAndProgress(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("garden", "root", "The garden"),
		obj("rose", "garden", "Rose"), obj("fern", "garden", "Fern"), obj("oak", "garden", "Oak"),
	)

	view := viewOf([]models.Objective{rows[4]}, rows)
	view.Frontier.Status["rose"] = navigation.StatusComplete
	view.Frontier.Status["fern"] = navigation.StatusComplete

	quest := BuildQuestView(view)

	require.Len(t, quest.Places, 1)
	place := quest.Places[0]
	assert.Equal(t, "Complete all three", place.Rule)
	assert.Equal(t, 2, place.Found)
	assert.Equal(t, 3, place.Total)
	assert.False(t, place.CanFinish, "a band with no range offers no choice")
}

// A band past nine children says the numeral: "Complete all several" is not a
// rule a player can follow.
func TestQuestView_BandPastNineChildrenUsesTheNumeral(t *testing.T) {
	rule := questRule(models.Objective{Routing: models.RouteStrategyFreeRoam}, 12, 12)
	assert.Equal(t, "Complete all 12 · in any order", rule)
}

// A finishable root is the quest-level press: the root draws no place of its
// own, so the affordance lands in the loose group at the top.
func TestQuestView_FinishableRootStaysAtTheTop(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("intro", "root", "Say hello"))
	rows[0].ChildrenMin = intPtr(1)
	rows[0].ChildrenMax = intPtr(3)

	view := viewOf([]models.Objective{rows[0], rows[1]}, rows)
	view.Frontier.Status["root"] = navigation.StatusFinishable

	quest := BuildQuestView(view)

	require.Len(t, quest.Places, 1)
	assert.False(t, quest.Places[0].IsSection)
	assert.True(t, quest.Places[0].CanFinish)
	assert.Equal(t, "root", quest.Places[0].FinishSlug)
}

// A quest whose tree failed to load must not lose its rows: with no tree there
// are no sections to find, but every row still has to appear.
func TestQuestView_SurvivesAMissingTree(t *testing.T) {
	orphan := obj("lost", "nowhere", "Still a task")

	view := BuildQuestView(viewOf([]models.Objective{orphan}, nil))

	require.Len(t, view.Places, 1)
	assert.False(t, view.Places[0].IsSection)
	assert.Equal(t, "Still a task", view.Places[0].Rows[0].Title)
}

// A parent chain that loops must not hang the page, and must not swallow the
// row: storage can hold a cycle and lint reports one, but the work underneath
// it is still work the run can do.
func TestQuestView_SurvivesAParentCycle(t *testing.T) {
	rows := tree(obj("a", "b", "A"), obj("b", "a", "B"), obj("leaf", "a", "A task"))

	done := make(chan QuestView, 1)
	go func() { done <- BuildQuestView(viewOf([]models.Objective{rows[2]}, rows)) }()

	select {
	case view := <-done:
		require.Len(t, view.Places, 1)
		assert.Equal(t, "A task", view.Places[0].Rows[0].Title)
	case <-time.After(2 * time.Second):
		t.Fatal("BuildQuestView did not return: the parent walk has no guard")
	}
}

// The header counts against what the run still owes, not the authored tree.
func TestMetricFor_ProgressIsDoneAgainstWhatIsStillOwed(t *testing.T) {
	progress := navigation.Progress{Done: 11, Remaining: 4}

	got := metricFor(models.QuestSettings{}, progress)

	assert.True(t, got.Show)
	assert.True(t, got.HasTotal)
	assert.Equal(t, 11, got.Done)
	assert.Equal(t, 15, got.Total, "done plus what the run still owes")
	assert.Equal(t, "done", got.Label)
}

// Count has no target, so it cannot disagree with a total.
func TestMetricFor_FoundHasNoTotal(t *testing.T) {
	got := metricFor(models.QuestSettings{ProgressFeedback: game.ProgressFeedbackCount}, navigation.Progress{Done: 3, Remaining: 6})

	assert.True(t, got.Show)
	assert.False(t, got.HasTotal)
	assert.Equal(t, 3, got.Done)
	assert.Equal(t, "found", got.Label)
}

func TestMetricFor_NoneAndAnEmptyTotalSayNothing(t *testing.T) {
	assert.False(t, metricFor(models.QuestSettings{ProgressFeedback: game.ProgressFeedbackNone}, navigation.Progress{Done: 3, Remaining: 6}).Show)
	assert.False(t, metricFor(models.QuestSettings{}, navigation.Progress{}).Show, "no denominator to show")
}

func headerRun(metric game.ProgressFeedback, points bool) models.Run {
	return models.Run{
		Name:   "The Quokkas",
		Points: 40,
		Quest:  models.Quest{Name: "The Perfumer's Garden", Settings: models.QuestSettings{EnablePoints: points, ProgressFeedback: metric}},
	}
}

func TestQuestHeader_ShowsTheMetricTheAuthorChose(t *testing.T) {
	view := &services.PlayerObjectiveView{Progress: navigation.Progress{Done: 3, Remaining: 6}}

	html := render(t, questHeader(ObjectivesParams{Run: headerRun(game.ProgressFeedbackFraction, false), View: view}))
	assert.Contains(t, html, "The Perfumer&#39;s Garden")
	assert.Contains(t, html, "The Quokkas")
	assert.Contains(t, html, `aria-label="3 of 9 done"`)
	assert.NotContains(t, html, "<progress", "the bar is gone from every page")

	html = render(t, questHeader(ObjectivesParams{Run: headerRun(game.ProgressFeedbackCount, false), View: view}))
	assert.Contains(t, html, `aria-label="3 found"`)
	assert.NotContains(t, html, "/9")

	html = render(t, questHeader(ObjectivesParams{Run: headerRun(game.ProgressFeedbackNone, false), View: view}))
	assert.NotContains(t, html, "aria-label=")
}

// Points are the run's own score. Rank is a comparison and is not on this page.
func TestQuestHeader_ShowsPointsOnlyWhenOnAndNeverRank(t *testing.T) {
	view := &services.PlayerObjectiveView{Progress: navigation.Progress{Done: 1, Remaining: 1}}

	// The metric off, so the only " of " that could appear is a rank.
	on := render(t, questHeader(ObjectivesParams{Run: headerRun(game.ProgressFeedbackNone, true), View: view}))
	assert.Contains(t, on, "40 pts")
	assert.NotContains(t, on, " of ", "no position among other teams")

	off := render(t, questHeader(ObjectivesParams{Run: headerRun("", false), View: view}))
	assert.NotContains(t, off, "pts")
}

// A run with no name is told so, not shown a blank.
func TestQuestHeader_NamesAnUnnamedTeam(t *testing.T) {
	run := headerRun("", false)
	run.Name = ""

	html := render(t, questHeader(ObjectivesParams{Run: run, View: &services.PlayerObjectiveView{}}))
	assert.Contains(t, html, "No name yet")
}

// A row carries the description under its title, which is what the second
// field is for: the name is read in a list, the description is read standing
// in front of the thing.
func TestQuestView_RowCarriesItsDescription(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("chest", "root", "Find the plan chest"))
	rows[1].Description = "Level 2, past the model-making room."

	view := BuildQuestView(viewOf([]models.Objective{rows[1]}, rows))

	assert.Equal(t, "Level 2, past the model-making room.", view.Places[0].Rows[0].Description)
}

// An author's pass-through is the same cut made on purpose: the flagged section
// has no place on the screen, and what sat below it is drawn in its stead.
func TestQuestView_AFlaggedSectionPassesThrough(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("trip", "root", "The trip"),
		obj("birds", "trip", "Birds"),
		obj("owl", "birds", "Find the owl"),
	)
	rows[2].PassThrough = true

	view := BuildQuestView(viewOf([]models.Objective{rows[3]}, rows))

	require.Len(t, view.Places, 1)
	assert.Equal(t, "The trip", view.Places[0].Title, "the pool is not a place")
	assert.Empty(t, view.Places[0].Path)
	require.Len(t, view.Places[0].Rows, 1)
	assert.Equal(t, "Find the owl", view.Places[0].Rows[0].Title)
}

// A section's colour is its place in the authored tree, so unlocking another
// never recolours it.
func TestSectionHues_AreFixedByPositionNotByWhatIsUnlocked(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("a", "root", "A"), obj("a1", "a", "A1"),
		obj("b", "root", "B"), obj("b1", "b", "B1"),
		obj("c", "root", "C"), obj("c1", "c", "C1"),
	)

	hues := sectionHues(rows)

	assert.Equal(t, HueFor(0), hues["a"])
	assert.Equal(t, HueFor(1), hues["b"])
	assert.Equal(t, HueFor(2), hues["c"])
	assert.NotContains(t, hues, "a1", "a task is not a section")
	assert.NotContains(t, hues, "root")

	// Only C offered: it keeps the colour it has when everything is.
	view := BuildQuestView(viewOf([]models.Objective{rows[6]}, rows))
	require.Len(t, view.Places, 1)
	assert.Equal(t, HueFor(2), view.Places[0].Hue)
}

// A pass-through section is not drawn: no colour, no number.
func TestSectionHues_PassThroughSectionsTakeNoNumber(t *testing.T) {
	pass := obj("p", "root", "P")
	pass.PassThrough = true
	rows := tree(
		obj("root", "", "Quest"),
		pass, obj("p1", "p", "P1"),
		obj("b", "root", "B"), obj("b1", "b", "B1"),
	)

	hues := sectionHues(rows)

	assert.NotContains(t, hues, "p")
	assert.Equal(t, HueFor(0), hues["b"])
}

// Groups join neighbours under the outermost section and drop it from the path;
// deeper ancestors stay as the crumb.
func TestQuestView_GroupsPlacesUnderTheirOutermostSection(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("scent", "root", "Make your scent"),
		obj("top", "scent", "Top note"), obj("t1", "top", "Bergamot"),
		obj("base", "scent", "Base note"), obj("b1", "base", "Oakmoss"),
		obj("still", "root", "The still room"), obj("s1", "still", "Steam"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[3], rows[5], rows[7]}, rows))
	groups := view.Groups()

	require.Len(t, groups, 2)
	assert.Equal(t, "Make your scent", groups[0].Title)
	require.Len(t, groups[0].Places, 2)
	assert.Equal(t, "Top note", groups[0].Places[0].Title)
	assert.Empty(t, groups[0].Places[0].Path, "the group is the heading, so it is not also the crumb")
	assert.Empty(t, groups[1].Title, "a section directly under the quest has no group above it")
}

func TestQuestPill_SaysTheCountWhereTheBandAsksForSeveral(t *testing.T) {
	two := 3
	section := models.Objective{ChildrenMin: &two}
	band := game.FillBand(section.ChildrenMin, section.ChildrenMax, 5)

	assert.Equal(t, "1 of 3 done", questPill(section, band, 5, 1))
	assert.Equal(t, "3 of 3 done", questPill(section, band, 5, 4), "never past the band's own minimum")
}

// Free roam is never said: any order is the only order on offer.
func TestQuestPill_NeverSaysInAnyOrder(t *testing.T) {
	section := models.Objective{Routing: models.RouteStrategyFreeRoam}
	band := game.FillBand(nil, nil, 3)

	assert.Equal(t, "0 of 3 done", questPill(section, band, 3, 0), "all three is a count to watch")
	assert.NotContains(t, questPill(models.Objective{Routing: models.RouteStrategyFreeRoam}, game.FillBand(nil, nil, 1), 1, 0), "any order")
}

func TestQuestPill_OrderedIsNextUp(t *testing.T) {
	section := models.Objective{Routing: models.RouteStrategyOrdered}

	assert.Equal(t, "Next up", questPill(section, game.FillBand(nil, nil, 3), 3, 0))
}

// A ceiling below the child count is a pick: "Complete all three" would say keep
// going after the section had shut.
func TestBandWords_ACeilingBelowTheChildCountIsAPick(t *testing.T) {
	one := 1
	band := game.FillBand(&one, &one, 3)

	assert.Equal(t, "Pick one", bandWords(band, 3))
	assert.Equal(t, "Complete all three", bandWords(game.FillBand(nil, nil, 3), 3))
}

func finishablePlace() QuestPlace {
	return QuestPlace{
		IsSection: true, Title: "The still room", CanFinish: true,
		FinishLabel: "That's enough for now", FinishSlug: "still",
		Rows: []QuestRow{{Slug: "steam", Title: "Steam distillation"}},
	}
}

// Tasks still on offer come first, then an "or", then the press, which asks
// before it acts.
func TestQuestPlace_MetBandOffersTheRestThenOrThenAConfirmedPress(t *testing.T) {
	html := render(t, questPlace(finishablePlace(), false))

	task := strings.Index(html, "Steam distillation")
	or := strings.Index(html, `class="q-or"`)
	press := strings.Index(html, `class="q-fin"`)
	require.True(t, task >= 0 && or > task && press > or, "task, then or, then the press")

	assert.Contains(t, html, `<dialog id="finish-still"`)
	assert.Contains(t, html, "Finish The still room?")
	assert.Contains(t, html, "This closes the section and skips what is left.")
	form := strings.Index(html, `action="/objective/still/finish"`)
	assert.Greater(t, form, strings.Index(html, "<dialog"), "the post is in the dialog, so the press alone sends nothing")
	assert.Contains(t, html, `hx-boost="true"`, "boosted, so the list that follows is swapped in and not reloaded")
	assert.NotContains(t, html, "hx-post", "an hx-post would answer a redirect with a full page load")
	assert.NotContains(t, html, "You have done enough to finish")
}

func TestQuestPlace_NoPressUntilTheBandIsMet(t *testing.T) {
	place := finishablePlace()
	place.CanFinish = false

	html := render(t, questPlace(place, false))

	assert.NotContains(t, html, "q-fin")
	assert.NotContains(t, html, "<dialog")
}

// The pill carries the count, so no separate "found" footer is drawn.
func TestQuestPlace_WearsItsPillAndTheSectionsHue(t *testing.T) {
	place := finishablePlace()
	place.CanFinish = false
	place.Pill = "1 of 3 done"
	place.Hue = HueFor(2)

	html := render(t, questPlace(place, true))

	assert.Contains(t, html, `class="q-pill">1 of 3 done`)
	assert.Contains(t, html, hueClass(HueFor(2)))
	assert.Contains(t, html, "q-head-sub", "under a group the heading steps down and takes a dot")
	assert.NotContains(t, html, "found")
}

func TestQuestRow_OnlyAGuidedFirstRowIsTheHero(t *testing.T) {
	row := QuestRow{Slug: "bench", Title: "The blending bench", IsDoor: true}

	assert.Contains(t, render(t, questRow(row, true)), "q-hero")
	assert.Contains(t, render(t, questRow(row, true)), "Next up")
	assert.NotContains(t, render(t, questRow(row, false)), "q-hero")
}
