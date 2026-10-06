package templates

import (
	"testing"

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

// viewOf builds the service output the grouping takes.
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

// The frontier omits an open section, so the view must put it back or three
// parts of one task read as three tasks.
func TestQuestView_RebuildsTheContainerTheFrontierFlattened(t *testing.T) {
	rows := tree(
		obj("root", "", "The Perfumer's Garden"),
		obj("top", "root", "Find a top note"),
		obj("bergamot", "top", "Bergamot"),
		obj("pepper", "top", "Pink pepper"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[2], rows[3]}, rows))

	require.Len(t, view.Groups, 1, "both rows came out of one section")
	group := view.Groups[0]
	assert.Equal(t, "Find a top note", group.Title)
	assert.Equal(t, "top", group.ID)
	require.Len(t, group.Rows, 2)
	assert.Equal(t, "Bergamot", group.Rows[0].Title)
	assert.Equal(t, "Pink pepper", group.Rows[1].Title)
}

// Wrapping a row under the root would invent a container the author never wrote.
func TestQuestView_TopLevelRowsStandAlone(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("intro", "root", "Find out how to play"),
		obj("top", "root", "Find a top note"),
		obj("bergamot", "top", "Bergamot"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[1], rows[3]}, rows))

	require.Len(t, view.Groups, 2)
	assert.False(t, view.Groups[0].IsSection, "a row under the root is its own task")
	assert.Equal(t, "Find out how to play", view.Groups[0].Rows[0].Title)
	assert.True(t, view.Groups[1].IsSection)
	assert.Equal(t, "Find a top note", view.Groups[1].Title)
}

// Re-sorting would put the quest in an order the author never arranged.
func TestQuestView_KeepsTheFrontierOrder(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("a", "root", "Section A"), obj("a1", "a", "A one"),
		obj("b", "root", "Section B"), obj("b1", "b", "B one"),
	)

	view := BuildQuestView(viewOf([]models.Objective{rows[4], rows[2]}, rows))

	require.Len(t, view.Groups, 2)
	assert.Equal(t, "Section B", view.Groups[0].Title, "B's row came first, so B's group does")
	assert.Equal(t, "Section A", view.Groups[1].Title)
}

// An objective with no proof is a door: the row must not promise work.
func TestQuestView_RowWithoutProofIsADoor(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("read", "root", "Read the safety notice"))

	assert.True(t, BuildQuestView(viewOf([]models.Objective{rows[1]}, rows)).Groups[0].Rows[0].IsDoor)

	withProof := viewOf([]models.Objective{rows[1]}, rows)
	withProof.HasProof = map[string]bool{"read": true}
	assert.False(t, BuildQuestView(withProof).Groups[0].Rows[0].IsDoor)
}

// The icon comes from the first proof block, so no one has to classify
// objectives into kinds.
func TestQuestView_RowBorrowsItsProofBlocksIcon(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("gate", "root", "Scan the tag on the plan chest"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.FirstProofBlock = map[string]string{"gate": "scan"}

	icon := BuildQuestView(view).Groups[0].Rows[0].IconSVG
	assert.Contains(t, icon, "<svg")
	assert.Contains(t, icon, "scan-qr-code")
}

// With no named block the row draws its own marker rather than a misleading one.
func TestQuestView_RowWithNoNamedBlockHasNoIcon(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("read", "root", "Read the notice"))

	view := BuildQuestView(viewOf([]models.Objective{rows[1]}, rows))
	assert.Empty(t, view.Groups[0].Rows[0].IconSVG)
}

// A finishable section is a row in its own right; its button is its only control.
func TestQuestView_FinishableSectionCarriesItsButton(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("wing", "root", "Visit the east wing"))
	rows[1].FinishLabel = "Leave the wing"

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	row := BuildQuestView(view).Groups[0].Rows[0]
	assert.True(t, row.CanFinish)
	assert.Equal(t, "Leave the wing", row.FinishLabel)
}

// A blank label would render an empty button.
func TestQuestView_FinishLabelFallsBackToPlainWords(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("wing", "root", "Visit the east wing"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	assert.Equal(t, "Finish", BuildQuestView(view).Groups[0].Rows[0].FinishLabel)
}

// Without the tree there are no sections, but every row must still appear.
func TestQuestView_SurvivesAMissingTree(t *testing.T) {
	orphan := obj("lost", "nowhere", "Still a task")

	view := BuildQuestView(viewOf([]models.Objective{orphan}, nil))

	require.Len(t, view.Groups, 1)
	assert.False(t, view.Groups[0].IsSection)
	assert.Equal(t, "Still a task", view.Groups[0].Rows[0].Title)
}

// The bar counts finished objectives against the whole quest, not the frontier.
func TestQuestChrome_CountsTheQuestNotTheFrontier(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("a", "root", "A"), obj("b", "root", "B"), obj("c", "root", "C"),
	)
	view := &services.PlayerObjectiveView{
		Objectives: rows,
		Frontier: navigation.Frontier{Status: map[string]navigation.Status{
			"a": navigation.StatusComplete,
			"b": navigation.StatusAvailable,
			"c": navigation.StatusLocked,
		}},
	}

	chrome := QuestChrome(models.Run{}, view)
	assert.Equal(t, 1, chrome.Done)
	assert.Equal(t, 3, chrome.Total, "the root is the quest, not a step in it")
	assert.Equal(t, "quest", chrome.Active)
}
