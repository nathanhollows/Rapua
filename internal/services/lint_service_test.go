package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// lintFixture is a quest built from rows, the way the editor leaves one.
type lintFixture struct {
	svc     *services.LintService
	dbc     *bun.DB
	questID string
}

func setupLintService(t *testing.T) (lintFixture, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	ctx := context.Background()

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	svc := services.NewLintService(
		repositories.NewQuestRepository(dbc),
		repositories.NewQuestSettingsRepository(dbc),
		repositories.NewObjectiveRepository(dbc),
		repositories.NewBlockRepository(dbc, blockStateRepo),
		blocks.Registry(),
	)

	userID := gofakeit.UUID()
	insertTestUser(t, dbc, userID)
	quest := &models.Quest{ID: gofakeit.UUID(), Name: "Lint Test", UserID: userID}
	_, err := dbc.NewInsert().Model(quest).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(&models.QuestSettings{QuestID: quest.ID}).Exec(ctx)
	require.NoError(t, err)

	return lintFixture{svc: svc, dbc: dbc, questID: quest.ID}, cleanup
}

// insert writes a row the way the editor would leave one, which includes the
// routing every objective is created with. A fixture without it builds a shape
// no quest contains and hides whatever fires on the real one.
func (f lintFixture) insert(t *testing.T, obj models.Objective) models.Objective {
	t.Helper()
	if obj.Routing == "" {
		obj.Routing = models.RouteStrategyOrdered
	}
	return f.insertLegacy(t, obj)
}

// insertLegacy writes a row verbatim, for the states only an older backfill or
// a hand-edited database can produce.
func (f lintFixture) insertLegacy(t *testing.T, obj models.Objective) models.Objective {
	t.Helper()
	if obj.ID == "" {
		obj.ID = gofakeit.UUID()
	}
	obj.QuestID = f.questID
	if obj.Title == "" {
		obj.Title = obj.Slug
	}
	_, err := f.dbc.NewInsert().Model(&obj).Exec(context.Background())
	require.NoError(t, err)
	return obj
}

// insertBlock writes a block straight onto an owner, the way the block editor
// leaves one.
func (f lintFixture) insertBlock(t *testing.T, ownerID, blockType string, points int) {
	t.Helper()
	block := models.Block{
		ID: gofakeit.UUID(), OwnerID: ownerID, Type: blockType,
		Context: game.ContextObjectiveProof, Points: points, Data: []byte("{}"),
	}
	_, err := f.dbc.NewInsert().Model(&block).Exec(context.Background())
	require.NoError(t, err)
}

// lint flattens the run: most tests here ask whether a rule fired at all,
// rather than which row it landed on. TestLintService_DiagnosticsLandOnTheirObjective
// covers that.
func (f lintFixture) lint(t *testing.T) game.LintResult {
	t.Helper()
	return f.lintQuest(t).All()
}

func (f lintFixture) lintQuest(t *testing.T) services.QuestLint {
	t.Helper()
	result, err := f.svc.LintQuest(context.Background(), f.questID)
	require.NoError(t, err)
	return result
}

// A quest the editor could plausibly have produced says nothing alarming.
func TestLintService_WellFormedQuest(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
	f.insert(t, models.Objective{Slug: "one", ParentID: root.ID, Position: 0})
	f.insert(t, models.Objective{Slug: "two", ParentID: root.ID, Position: 1})

	result := f.lint(t)
	for _, e := range result.Errors {
		t.Errorf("unexpected error %s at %s: %s", e.Code, e.Path, e.Message)
	}
	assert.True(t, result.IsValid())
}

// The point of the exercise: rules written for documents now reach the tree an
// author built by clicking. A band above the child count is one nobody can
// meet, and until now only an import would have said so.
func TestLintService_DocumentRulesReachStoredRows(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	minChildren := 3
	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
	section := f.insert(t, models.Objective{
		Slug: "section", ParentID: root.ID, Routing: models.RouteStrategyFreeRoam,
		ChildrenMin: &minChildren,
	})
	f.insert(t, models.Objective{Slug: "only-child", ParentID: section.ID})

	assert.True(t, f.lint(t).HasError("BAND_OUT_OF_RANGE"))
}

// Rows reach shapes a document cannot: a document is a tree by construction, so
// none of these four have a rule in the document's own grammar.
func TestLintService_ShapesOnlyRowsCanReach(t *testing.T) {
	t.Run("no root", func(t *testing.T) {
		f, cleanup := setupLintService(t)
		defer cleanup()

		orphan := f.insert(t, models.Objective{Slug: "adrift"})
		_, err := f.dbc.NewUpdate().Model((*models.Objective)(nil)).
			Set("parent_id = ?", orphan.ID).Where("id = ?", orphan.ID).
			Exec(context.Background())
		require.NoError(t, err)

		assert.True(t, f.lint(t).HasError(services.LintNoRoot))
	})

	t.Run("several roots", func(t *testing.T) {
		f, cleanup := setupLintService(t)
		defer cleanup()

		f.insert(t, models.Objective{Slug: "one-root", Position: 0})
		f.insert(t, models.Objective{Slug: "another-root", Position: 1})

		result := f.lint(t)
		assert.True(t, result.HasError(services.LintSeveralRoots))
		assert.Contains(t, result.Errors[0].Message, "another-root",
			"the message names them, since the fix is to pick one")
	})

	t.Run("orphan", func(t *testing.T) {
		f, cleanup := setupLintService(t)
		defer cleanup()

		f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
		f.insert(t, models.Objective{Slug: "stranded", ParentID: gofakeit.UUID()})

		assert.True(t, f.lint(t).HasError(services.LintOrphaned))
	})

	t.Run("parent cycle", func(t *testing.T) {
		f, cleanup := setupLintService(t)
		defer cleanup()

		f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
		first := f.insert(t, models.Objective{Slug: "first", ParentID: gofakeit.UUID()})
		second := f.insert(t, models.Objective{Slug: "second", ParentID: first.ID})

		ctx := context.Background()
		_, err := f.dbc.NewUpdate().Model((*models.Objective)(nil)).
			Set("parent_id = ?", second.ID).Where("id = ?", first.ID).Exec(ctx)
		require.NoError(t, err)

		assert.True(t, f.lint(t).HasError(services.LintParentCycle))
	})
}

// A missing or ambiguous root stops the document rules running, because they
// walk down from one. Reporting a band on a quest nobody can open buries the
// thing that actually needs fixing.
func TestLintService_StructuralProblemsComeAlone(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	f.insert(t, models.Objective{Slug: "one-root", Position: 0})
	f.insert(t, models.Objective{Slug: "another-root", Position: 1})

	result := f.lint(t)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, services.LintSeveralRoots, result.Errors[0].Code)
}

// The reason for grouping at all: a message with a document path in it does
// not tell an author which row to go and fix.
func TestLintService_DiagnosticsLandOnTheirObjective(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	minChildren := 3
	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
	// Deep enough that the diagnostic's path has to be trimmed through more
	// than one level to find the row it belongs to.
	outer := f.insert(t, models.Objective{
		Slug: "outer", ParentID: root.ID, Routing: models.RouteStrategyFreeRoam,
	})
	inner := f.insert(t, models.Objective{
		Slug: "inner", ParentID: outer.ID, Routing: models.RouteStrategyFreeRoam,
		ChildrenMin: &minChildren,
	})
	f.insert(t, models.Objective{Slug: "only-child", ParentID: inner.ID})

	result := f.lintQuest(t)

	assert.True(t, result.For(inner.ID).HasError("BAND_OUT_OF_RANGE"),
		"the band is wrong on inner, so that is the row that should be marked")
	assert.Empty(t, result.For(outer.ID).Errors,
		"trimming the path must stop at the objective that owns the field, not walk past it")
	assert.Empty(t, result.Quest.Errors,
		"a problem with a row is not a problem with the quest")
	assert.Equal(t, 1, result.ObjectivesWithErrors())
}

// A structural problem still names its row even though no document was built
// to give it a path: the orphan is exactly the row somebody has to drag back.
func TestLintService_StructuralProblemsNameTheirObjective(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
	stranded := f.insert(t, models.Objective{Slug: "stranded", ParentID: gofakeit.UUID()})

	result := f.lintQuest(t)
	assert.True(t, result.For(stranded.ID).HasError(services.LintOrphaned))
}

// Not everything belongs to a row. The root has no card in the editor, so what
// the rules say about it has nowhere to hang and stays with the quest.
func TestLintService_QuestLevelDiagnosticsStayWithTheQuest(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam, Draft: true})
	f.insert(t, models.Objective{Slug: "child", ParentID: root.ID})

	result := f.lintQuest(t)
	assert.True(t, result.Quest.HasError("ROOT_DRAFT"))
	assert.Empty(t, result.For(root.ID).Errors)
}

// A quest built by clicking, with nothing chosen that the editor does not
// force. This is the shape every test here should have started from: setting
// routing explicitly on every node hid an error that fires on every real quest.
func TestLintService_DefaultQuestIsClean(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyOrdered})
	f.insert(t, models.Objective{Slug: "one", ParentID: root.ID, Position: 0, Routing: models.RouteStrategyOrdered})
	f.insert(t, models.Objective{Slug: "two", ParentID: root.ID, Position: 1, Routing: models.RouteStrategyOrdered})

	result := f.lint(t)
	for _, e := range result.Errors {
		t.Errorf("unexpected error %s at %s: %s", e.Code, e.Path, e.Message)
	}
	// Warnings too. A panel that cries wolf on every quest is one nobody
	// reads, and asserting only on errors let exactly that ship.
	for _, w := range result.Warnings {
		t.Errorf("unexpected warning %s at %s: %s", w.Code, w.Path, w.Message)
	}
}

// Routing left empty is an unmade decision, not a default. It reached the
// runtime as free roam, which is a gameplay choice nobody made.
func TestLintService_UnsetRoutingIsAnError(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insertLegacy(t, models.Objective{Slug: "root", Routing: ""})
	f.insert(t, models.Objective{Slug: "child", ParentID: root.ID})

	result := f.lint(t)
	require.True(t, result.HasError("INVALID_ROUTING"))
	assert.Contains(t, result.Errors[0].Message, "not set",
		"an omission reads differently from a typo, and the fix is different too")
}

// The band guard and lint have to count the same children. They did not: the
// guard counted every row and lint counted the published ones, so a band saved
// while its siblings were parked turned red the moment one was published.
func TestLintService_BandGuardAndLintCountTheSameChildren(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	minChildren := 2
	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyOrdered})
	section := f.insert(t, models.Objective{
		Slug: "section", ParentID: root.ID, Routing: models.RouteStrategyOrdered,
		ChildrenMin: &minChildren,
	})
	f.insert(t, models.Objective{Slug: "published", ParentID: section.ID, Position: 0})
	f.insert(t, models.Objective{Slug: "parked", ParentID: section.ID, Position: 1, Draft: true})

	// One published child against a minimum of two: lint says so, and the
	// guard must agree rather than having accepted this on a count of two.
	assert.True(t, f.lint(t).HasError("BAND_OUT_OF_RANGE"))
}

// Points rules have to fire on a quest built in the editor, not only on one
// arriving as JSON. They did not: the builder path emitted a Go int while the
// rules type-switch the JSON number types, so the two gates disagreed about
// byte-identical content and every points rule was dead in the editor.
func TestLintService_PointsRulesReachStoredBlocks(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyOrdered})
	child := f.insert(t, models.Objective{Slug: "child", ParentID: root.ID})
	f.insertBlock(t, child.ID, "checklist", -5)

	assert.True(t, f.lint(t).HasError("INVALID_POINTS"))
}

// An orphan is recoverable: the tree renders it at the top level so it can be
// dragged back. Suppressing every other diagnostic over one such row would
// hide the whole quest behind a problem the author can fix in a drag.
func TestLintService_OrphanDoesNotSuppressTheRest(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	minChildren := 3
	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyOrdered})
	section := f.insert(t, models.Objective{
		Slug: "section", ParentID: root.ID, Routing: models.RouteStrategyOrdered,
		ChildrenMin: &minChildren,
	})
	f.insert(t, models.Objective{Slug: "only-child", ParentID: section.ID})
	f.insert(t, models.Objective{Slug: "stranded", ParentID: gofakeit.UUID()})

	result := f.lint(t)
	assert.True(t, result.HasError(services.LintOrphaned), "the orphan is still reported")
	assert.True(t, result.HasError("BAND_OUT_OF_RANGE"),
		"and so is everything the document rules would have said without it")
}

// Diagnostics resolve through the document path, not the slug. Storage keeps
// slugs unique per quest (objectives_quest_slug), so a slug map happens to be
// safe today, but the path is what the linter actually addresses rows by and
// it stays unique whatever slugs do.
func TestLintService_AttributionSurvivesIdenticalTitles(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyOrdered})
	first := f.insert(t, models.Objective{
		Slug: "first", Title: "Same Name", ParentID: root.ID, Position: 0, Routing: "bogus",
	})
	second := f.insert(t, models.Objective{
		Slug: "second", Title: "Same Name", ParentID: root.ID, Position: 1,
	})
	f.insert(t, models.Objective{Slug: "under-first", ParentID: first.ID})

	result := f.lintQuest(t)
	assert.True(t, result.For(first.ID).HasError("INVALID_ROUTING"))
	assert.False(t, result.For(second.ID).HasError("INVALID_ROUTING"))
}

// A run that could not happen is not a clean bill of health. Callers gate on
// IsValid, and the zero value of a lint result is "nothing found".
func TestQuestLint_UnavailableIsNotValid(t *testing.T) {
	assert.False(t, services.QuestLint{Unavailable: true}.IsValid(),
		"an unread quest must not pass for a sound one")
	assert.True(t, services.QuestLint{}.IsValid(),
		"and a run that found nothing still passes")
}

// Only the rows that close the loop are its own ancestors. Everything below
// one is unreachable too, but its author cannot fix it, and twenty copies of
// the wrong message bury the two rows that are actually wrong.
func TestLintService_CycleBlamesOnlyTheRowsInIt(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	f.insert(t, models.Objective{Slug: "root"})
	first := f.insert(t, models.Objective{Slug: "first", ParentID: gofakeit.UUID(), Position: 0})
	second := f.insert(t, models.Objective{Slug: "second", ParentID: first.ID, Position: 0})
	below := f.insert(t, models.Objective{Slug: "below", ParentID: second.ID, Position: 0})
	deeper := f.insert(t, models.Objective{Slug: "deeper", ParentID: below.ID, Position: 0})

	// Close the loop: first now hangs off second, which hangs off first.
	_, err := f.dbc.NewUpdate().Model((*models.Objective)(nil)).
		Set("parent_id = ?", second.ID).Set("position = ?", 1).
		Where("id = ?", first.ID).Exec(context.Background())
	require.NoError(t, err)

	blamed := map[string]bool{}
	for _, e := range f.lint(t).Errors {
		if e.Code == services.LintParentCycle {
			blamed[strings.TrimPrefix(e.Path, "objective:")] = true
		}
	}

	assert.True(t, blamed["first"], "the rows that close the loop are named")
	assert.True(t, blamed["second"])
	assert.False(t, blamed[below.Slug], "the row below it is not its own ancestor")
	assert.False(t, blamed[deeper.Slug], "nor is anything under that")
	assert.Len(t, blamed, 2, "and nothing else is blamed")
}

// A stranded row is exactly what the repair UI puts in front of an author, and
// it used to arrive with no diagnostics at all: the document descended from the
// root, so nothing under an orphan was ever linted.
func TestLintService_OrphanSubtreeStillGetsContentDiagnostics(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	minChildren := 3
	f.insert(t, models.Objective{Slug: "root"})
	stranded := f.insert(t, models.Objective{
		Slug: "stranded", ParentID: gofakeit.UUID(), ChildrenMin: &minChildren,
	})
	f.insert(t, models.Objective{Slug: "only-child", ParentID: stranded.ID})

	result := f.lintQuest(t)
	assert.True(t, result.HasError(services.LintOrphaned), "it is still reported as stranded")
	assert.True(t, result.For(stranded.ID).HasError("BAND_OUT_OF_RANGE"),
		"and its band is checked like any other, on the row itself")
}

// MAX_NEXT_IGNORED belongs to import. The editor only shows the randomised
// window while routing is randomised, so a row carrying one under another
// strategy got there by import or by switching strategy afterwards: nothing on
// the page is wrong, and there is no control to clear. A document can still
// name both, which is worth saying at the point it is read.
func TestLintService_StaleRandomisedWindowIsNotReported(t *testing.T) {
	f, cleanup := setupLintService(t)
	defer cleanup()

	root := f.insert(t, models.Objective{Slug: "root", Routing: models.RouteStrategyFreeRoam})
	section := f.insert(t, models.Objective{
		Slug: "section", ParentID: root.ID,
		Routing: models.RouteStrategyOrdered, MaxNext: 3,
	})
	f.insert(t, models.Objective{Slug: "leaf", ParentID: section.ID})

	assert.False(t, f.lint(t).HasWarning("MAX_NEXT_IGNORED"))

	doc := game.GameDoc{
		Rapua: "v8", Name: "Doc",
		Structure: game.ObjectiveDoc{
			Slug: "root", Title: "Root", Routing: game.RouteStrategyFreeRoam,
			Children: []game.ObjectiveDoc{{
				Slug: "section", Title: "Section",
				Routing: game.RouteStrategyOrdered, MaxNext: 3,
				Children: []game.ObjectiveDoc{{Slug: "leaf", Title: "Leaf"}},
			}},
		},
	}
	assert.True(t, game.Lint(&doc, blocks.Registry()).HasWarning("MAX_NEXT_IGNORED"),
		"an import still reads the pair, where the document is the thing being judged")
}
