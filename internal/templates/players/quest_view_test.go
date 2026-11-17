package templates

import (
	"fmt"
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

// The author's tree has no depth limit; the screen has a fixed one.
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

// Wrapping it in a section would invent a container the author never wrote.
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

func TestQuestView_RowWithoutProofIsADoor(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("read", "root", "Read the safety notice"))

	assert.True(t, BuildQuestView(viewOf([]models.Objective{rows[1]}, rows)).Places[0].Rows[0].IsDoor)

	withProof := viewOf([]models.Objective{rows[1]}, rows)
	withProof.HasProof = map[string]bool{"read": true}
	assert.False(t, BuildQuestView(withProof).Places[0].Rows[0].IsDoor)
}

func TestQuestView_RowBorrowsItsProofBlocksIcon(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("gate", "root", "Scan the tag"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.FirstProofBlock = map[string]string{"gate": "scan"}

	icon := BuildQuestView(view).Places[0].Rows[0].IconSVG
	assert.Contains(t, icon, "scan-qr-code")
}

func intPtr(n int) *int { return &n }

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
	require.Len(t, place.Finishes, 1)
	assert.Equal(t, "Leave the wing", place.Finishes[0].Label)
	assert.Equal(t, "wing", place.Finishes[0].Slug)
	assert.Equal(t, "Visit the east wing", place.Finishes[0].Title)
	require.Len(t, place.Rows, 1, "its children stay tasks; the section itself is not one")
	assert.Equal(t, "Find the statue", place.Rows[0].Title)
	assert.Equal(t, 0, place.Found)
	assert.Equal(t, 1, place.Total)
	assert.Equal(t, "Complete at least one", place.Rule, "a ranged band says what the least is")
}

func TestQuestView_FinishLabelFallsBackToPlainWords(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("wing", "root", "Visit the east wing"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	assert.Equal(t, "Finish", BuildQuestView(view).Places[0].Finishes[0].Label)
}

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
	assert.Empty(t, place.Finishes, "a band with no range offers no choice")
}

func TestQuestView_BandPastNineChildrenUsesTheNumeral(t *testing.T) {
	rule := questRule(models.Objective{Routing: models.RouteStrategyFreeRoam}, 12, 12)
	assert.Equal(t, "Complete all 12 · in any order", rule)
}

// The root draws no place of its own, so the press lands in the loose group.
func TestQuestView_FinishableRootStaysAtTheTop(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("intro", "root", "Say hello"))
	rows[0].ChildrenMin = intPtr(1)
	rows[0].ChildrenMax = intPtr(3)

	view := viewOf([]models.Objective{rows[0], rows[1]}, rows)
	view.Frontier.Status["root"] = navigation.StatusFinishable

	quest := BuildQuestView(view)

	require.Len(t, quest.Places, 1)
	assert.False(t, quest.Places[0].IsSection)
	require.Len(t, quest.Places[0].Finishes, 1)
	assert.Equal(t, "root", quest.Places[0].Finishes[0].Slug)
	assert.True(t, quest.Places[0].Finishes[0].WholeQuest)
}

func TestQuestView_SurvivesAMissingTree(t *testing.T) {
	orphan := obj("lost", "nowhere", "Still a task")

	view := BuildQuestView(viewOf([]models.Objective{orphan}, nil))

	require.Len(t, view.Places, 1)
	assert.False(t, view.Places[0].IsSection)
	assert.Equal(t, "Still a task", view.Places[0].Rows[0].Title)
}

// Storage can hold a cycle, but the work under it is still work the run can do.
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

func TestMetricFor_ProgressIsDoneAgainstWhatIsStillOwed(t *testing.T) {
	progress := navigation.Progress{Done: 11, Remaining: 4}

	got := metricFor(models.QuestSettings{}, progress)

	assert.True(t, got.Show)
	assert.True(t, got.HasTotal)
	assert.Equal(t, 11, got.Done)
	assert.Equal(t, 15, got.Total, "done plus what the run still owes")
	assert.Equal(t, "done", got.Label)
}

func TestMetricFor_FoundHasNoTotal(t *testing.T) {
	got := metricFor(models.QuestSettings{ProgressFeedback: game.ProgressFeedbackCount}, navigation.Progress{Done: 5, Found: 3, Remaining: 6})

	assert.True(t, got.Show)
	assert.False(t, got.HasTotal)
	assert.Equal(t, 3, got.Done, "tasks found, not the sections that completed around them")
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
	view := &services.PlayerObjectiveView{Progress: navigation.Progress{Done: 3, Found: 3, Remaining: 6}}

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

func TestQuestHeader_ShowsPointsOnlyWhenOnAndNeverRank(t *testing.T) {
	view := &services.PlayerObjectiveView{Progress: navigation.Progress{Done: 1, Remaining: 1}}

	// The metric off, so the only " of " that could appear is a rank.
	on := render(t, questHeader(ObjectivesParams{Run: headerRun(game.ProgressFeedbackNone, true), View: view}))
	assert.Contains(t, on, "40 pts")
	assert.NotContains(t, on, " of ", "no position among other teams")

	off := render(t, questHeader(ObjectivesParams{Run: headerRun("", false), View: view}))
	assert.NotContains(t, off, "pts")
}

func TestQuestHeader_NamesAnUnnamedTeam(t *testing.T) {
	run := headerRun("", false)
	run.Name = ""

	html := render(t, questHeader(ObjectivesParams{Run: run, View: &services.PlayerObjectiveView{}}))
	assert.Contains(t, html, "No name yet")
}

func TestQuestView_RowCarriesItsDescription(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("chest", "root", "Find the plan chest"))
	rows[1].Description = "Level 2, past the model-making room."

	view := BuildQuestView(viewOf([]models.Objective{rows[1]}, rows))

	assert.Equal(t, "Level 2, past the model-making room.", view.Places[0].Rows[0].Description)
}

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

func TestSectionHues_ASectionPastTheDepthCapTakesNoColour(t *testing.T) {
	rows := tree(obj("root", "", "Quest"))
	parent := "root"
	for i := 0; i <= navigation.MaxDrawnDepth; i++ {
		id := fmt.Sprintf("s%d", i)
		rows = append(rows, obj(id, parent, id), obj(id+"-leaf", id, id+"-leaf"))
		parent = id
	}
	rows = append(rows, obj("next", "root", "Next"), obj("next-leaf", "next", "Leaf"))

	hues := sectionHues(rows)

	deepest := fmt.Sprintf("s%d", navigation.MaxDrawnDepth)
	assert.NotContains(t, hues, deepest, "never drawn, so it uses up no hue")
	assert.Equal(t, HueFor(navigation.MaxDrawnDepth), hues["next"], "the next drawn section takes the next unused hue")
}

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

func TestBandWords_ACeilingBelowTheChildCountIsAPick(t *testing.T) {
	one := 1
	band := game.FillBand(&one, &one, 3)

	assert.Equal(t, "Pick one", bandWords(band, 3))
	assert.Equal(t, "Complete all three", bandWords(game.FillBand(nil, nil, 3), 3))
}

func finishablePlace() QuestPlace {
	return QuestPlace{
		IsSection: true, Title: "The still room",
		Finishes: []QuestFinish{{Slug: "still", Label: "That's enough for now", Title: "The still room"}},
		Rows:     []QuestRow{{Slug: "steam", Title: "Steam distillation"}},
	}
}

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
	place.Finishes = nil

	html := render(t, questPlace(place, false))

	assert.NotContains(t, html, "q-fin")
	assert.NotContains(t, html, "<dialog")
}

func TestQuestPlace_WearsItsPillAndTheSectionsHue(t *testing.T) {
	place := finishablePlace()
	place.Finishes = nil
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

func TestQuestView_SectionWithRowsAndSubsectionsIsNamedOnce(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("scent", "root", "Make your scent"), obj("own", "scent", "Own task"),
		obj("top", "scent", "Top note"), obj("t1", "top", "Bergamot"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[2], rows[4]}, rows))
	groups := view.Groups()

	require.Len(t, groups, 1)
	assert.Equal(t, "Make your scent", groups[0].Title)
	require.Len(t, groups[0].Places, 2)
	assert.True(t, groups[0].Places[0].NamedByGroup, "the group heading already names the section")
	assert.False(t, groups[0].Places[1].NamedByGroup)
}

func TestQuestView_PassThroughFinishNamesItselfAndKeepsAnAncestorsPress(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("gallery", "root", "Gallery"),
		obj("bonus", "gallery", "Bonus"), obj("b1", "bonus", "Hidden"),
	)
	rows[2].PassThrough = true

	view := viewOf([]models.Objective{rows[1], rows[2]}, rows)
	view.Frontier.Status["gallery"] = navigation.StatusFinishable
	view.Frontier.Status["bonus"] = navigation.StatusFinishable

	place := BuildQuestView(view).Places[0]

	require.Len(t, place.Finishes, 2, "neither press overwrites the other")
	assert.Equal(t, "Gallery", place.Finishes[0].Title)
	assert.Equal(t, "Bonus", place.Finishes[1].Title)
	assert.Equal(t, "bonus", place.Finishes[1].Slug)
}
