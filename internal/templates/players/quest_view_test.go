package templates

import (
	"testing"
	"time"

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

// A finishable section arrives in the frontier as a row in its own right, and
// its finish button is the only control on it.
func TestQuestView_FinishableSectionCarriesItsButton(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("wing", "root", "Visit the east wing"))
	rows[1].FinishLabel = "Leave the wing"

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	row := BuildQuestView(view).Places[0].Rows[0]
	assert.True(t, row.CanFinish)
	assert.Equal(t, "Leave the wing", row.FinishLabel)
}

// An author who names no label still gets a button: the band decides one
// appears, and a blank label would render an empty one.
func TestQuestView_FinishLabelFallsBackToPlainWords(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("wing", "root", "Visit the east wing"))

	view := viewOf([]models.Objective{rows[1]}, rows)
	view.Frontier.Status["wing"] = navigation.StatusFinishable

	assert.Equal(t, "Finish", BuildQuestView(view).Places[0].Rows[0].FinishLabel)
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

// The bar measures the quest, not the frontier: what the run has finished
// against everything there is to finish.
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

// A row carries the description under its title, which is what the second
// field is for: the name is read in a list, the description is read standing
// in front of the thing.
func TestQuestView_RowCarriesItsDescription(t *testing.T) {
	rows := tree(obj("root", "", "Quest"), obj("chest", "root", "Find the plan chest"))
	rows[1].Description = "Level 2, past the model-making room."

	view := BuildQuestView(viewOf([]models.Objective{rows[1]}, rows))

	assert.Equal(t, "Level 2, past the model-making room.", view.Places[0].Rows[0].Description)
}
