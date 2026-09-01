package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// treeFixture builds root -> [alpha, beta], with alpha -> [alpha-one] beneath
// it: two levels, so parent-before-child ordering has something to get wrong.
type treeFixture struct {
	questID  string
	root     *models.Objective
	alpha    *models.Objective
	beta     *models.Objective
	alphaOne *models.Objective
}

func insertTree(t *testing.T, dbc *bun.DB) treeFixture {
	t.Helper()
	ctx := context.Background()
	parents := createTestParents(t, dbc)

	node := func(slug, parentID string, position int) *models.Objective {
		obj := &models.Objective{
			ID:       gofakeit.UUID(),
			QuestID:  parents.QuestID,
			ParentID: parentID,
			Position: position,
			Slug:     slug,
			Title:    slug,
		}
		_, err := dbc.NewInsert().Model(obj).Exec(ctx)
		require.NoError(t, err)
		return obj
	}

	root := node("root", "", 0)
	alpha := node("alpha", root.ID, 0)
	beta := node("beta", root.ID, 1)
	alphaOne := node("alpha-one", alpha.ID, 0)

	return treeFixture{parents.QuestID, root, alpha, beta, alphaOne}
}

func appendChild(t *testing.T, dbc *bun.DB, questID, parentID, slug string, position int) *models.Objective {
	t.Helper()
	obj := &models.Objective{
		ID: gofakeit.UUID(), QuestID: questID, ParentID: parentID,
		Position: position, Slug: slug, Title: slug,
	}
	_, err := dbc.NewInsert().Model(obj).Exec(context.Background())
	require.NoError(t, err)
	return obj
}

func slugsOf(objectives []models.Objective) []string {
	slugs := make([]string, len(objectives))
	for i, obj := range objectives {
		slugs[i] = obj.Slug
	}
	return slugs
}

func TestObjectiveRepository_FindTreeByQuestID_ParentsBeforeChildren(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.FindTreeByQuestID(context.Background(), fixture.questID)
	require.NoError(t, err)

	assert.Equal(t, []string{"root", "alpha", "alpha-one", "beta"}, slugsOf(got))
}

// A row whose parent is missing must still be returned. Dropping it would hide
// a partly-written tree instead of surfacing it.
func TestObjectiveRepository_FindTreeByQuestID_KeepsOrphans(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)

	orphan := &models.Objective{
		ID:       gofakeit.UUID(),
		QuestID:  fixture.questID,
		ParentID: "does-not-exist",
		Slug:     "orphan",
		Title:    "Orphan",
	}
	_, err := dbc.NewInsert().Model(orphan).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.FindTreeByQuestID(context.Background(), fixture.questID)
	require.NoError(t, err)

	assert.Len(t, got, 5)
	assert.Contains(t, slugsOf(got), "orphan")
}

func TestObjectiveRepository_FindRoot(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	root, err := repo.FindRoot(context.Background(), fixture.questID)
	require.NoError(t, err)
	assert.Equal(t, fixture.root.ID, root.ID)
}

func TestObjectiveRepository_FindChildren_InPositionOrder(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	children, err := repo.FindChildren(context.Background(), fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, slugsOf(children))

	grandchildren, err := repo.FindChildren(context.Background(), fixture.questID, fixture.alpha.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha-one"}, slugsOf(grandchildren))
}

func TestObjectiveRepository_Reposition_Reorders(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)
	ctx := context.Background()
	repo := repositories.NewObjectiveRepository(dbc)

	// Move beta ahead of alpha.
	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repo.Reposition(ctx, tx, fixture.beta.ID, fixture.root.ID, 0))
	require.NoError(t, tx.Commit())

	children, err := repo.FindChildren(ctx, fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"beta", "alpha"}, slugsOf(children))
	assert.Equal(t, 0, children[0].Position, "positions stay dense")
	assert.Equal(t, 1, children[1].Position)
}

// newPosition is the index the objective ends up at. A downward move within
// one parent is where that differs from "insert before the row currently at
// this index": moving the middle child to index 2 must actually put it last.
func TestObjectiveRepository_Reposition_DownwardMoveLandsAtTheGivenIndex(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)
	repo := repositories.NewObjectiveRepository(dbc)

	gamma := appendChild(t, dbc, fixture.questID, fixture.root.ID, "gamma", 2)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repo.Reposition(ctx, tx, fixture.alpha.ID, fixture.root.ID, 2))
	require.NoError(t, tx.Commit())

	children, err := repo.FindChildren(ctx, fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"beta", "gamma", "alpha"}, slugsOf(children))
	assert.Equal(t, gamma.ID, children[1].ID)
}

// An index past the end lands the objective last rather than failing: a caller
// working from a stale list should not lose the move.
func TestObjectiveRepository_Reposition_ClampsPastTheEnd(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)
	repo := repositories.NewObjectiveRepository(dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repo.Reposition(ctx, tx, fixture.alpha.ID, fixture.root.ID, 99))
	require.NoError(t, tx.Commit())

	children, err := repo.FindChildren(ctx, fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"beta", "alpha"}, slugsOf(children))
	assert.Equal(t, 1, children[1].Position, "positions stay dense after clamping")
}

// parent_id carries no foreign key, so a parent that does not exist would
// silently strand the subtree outside the walk from the root.
func TestObjectiveRepository_Reposition_RejectsAnUnknownParent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).
		Reposition(ctx, tx, fixture.alpha.ID, "no-such-objective", 0)
	require.ErrorIs(t, err, repositories.ErrParentNotInQuest)
}

// Dragging a section into its own child is reachable from any tree editor, and
// would detach both it and everything under it.
func TestObjectiveRepository_Reposition_RejectsMovingBeneathOwnDescendant(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).
		Reposition(ctx, tx, fixture.alpha.ID, fixture.alphaOne.ID, 0)
	require.ErrorIs(t, err, repositories.ErrParentIsDescendant)
}

func TestObjectiveRepository_Reposition_RejectsSelfAsParent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).
		Reposition(ctx, tx, fixture.alpha.ID, fixture.alpha.ID, 0)
	require.ErrorIs(t, err, repositories.ErrSelfParent)
}

// UpdateTx must not be able to move an objective: a sparsely built model would
// otherwise blank parent_id and detach the whole subtree into the root.
func TestObjectiveRepository_UpdateTx_LeavesThePlaceInTheTreeAlone(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)
	repo := repositories.NewObjectiveRepository(dbc)

	// A loaded row whose tree fields have been cleared, which is what a caller
	// building an update from partial form data would hand over.
	beta, err := repo.GetByID(ctx, fixture.beta.ID)
	require.NoError(t, err)
	beta.Title = "Renamed"
	beta.ParentID = ""
	beta.Position = 0

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repo.UpdateTx(ctx, tx, beta))
	require.NoError(t, tx.Commit())

	got, err := repo.GetByID(ctx, fixture.beta.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Title)
	assert.Equal(t, fixture.root.ID, got.ParentID, "parent survives a sparse update")
	assert.Equal(t, 1, got.Position, "position survives a sparse update")
}

func TestObjectiveRepository_Reposition_Reparents(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)
	ctx := context.Background()
	repo := repositories.NewObjectiveRepository(dbc)

	// Move alpha-one out from under alpha, up to the root.
	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repo.Reposition(ctx, tx, fixture.alphaOne.ID, fixture.root.ID, 1))
	require.NoError(t, tx.Commit())

	rootChildren, err := repo.FindChildren(ctx, fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "alpha-one", "beta"}, slugsOf(rootChildren))

	alphaChildren, err := repo.FindChildren(ctx, fixture.questID, fixture.alpha.ID)
	require.NoError(t, err)
	assert.Empty(t, alphaChildren, "the old parent closes the gap")
}

// The root has no siblings to renumber and no parent to move under, so moving
// it is a caller mistake rather than a no-op.
func TestObjectiveRepository_Reposition_RefusesTheRoot(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	fixture := insertTree(t, dbc)
	ctx := context.Background()

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).
		Reposition(ctx, tx, fixture.root.ID, fixture.alpha.ID, 0)
	require.ErrorIs(t, err, repositories.ErrCannotMoveRoot)
}

// The band's bounds are nullable because an explicit 0 is a different node from
// an omitted bound, so the distinction has to survive the database.
func TestObjectiveRepository_BandBoundsRoundTrip(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	parents := createTestParents(t, dbc)

	zero := 0
	three := 3
	obj := &models.Objective{
		ID:          gofakeit.UUID(),
		QuestID:     parents.QuestID,
		Slug:        "wing",
		Title:       "Wing",
		Color:       "primary",
		Routing:     "free_roam",
		ChildrenMin: &zero,
		ChildrenMax: &three,
		MaxNext:     2,
		FinishLabel: "Leave the wing",
	}
	_, err := dbc.NewInsert().Model(obj).Exec(ctx)
	require.NoError(t, err)

	got, err := repositories.NewObjectiveRepository(dbc).GetByID(ctx, obj.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ChildrenMin)
	assert.Equal(t, 0, *got.ChildrenMin, "an explicit zero must not read back as absent")
	require.NotNil(t, got.ChildrenMax)
	assert.Equal(t, 3, *got.ChildrenMax)
	assert.Equal(t, "primary", got.Color)
	assert.Equal(t, 2, got.MaxNext)
	assert.Equal(t, "Leave the wing", got.FinishLabel)
}

func TestObjectiveRepository_OmittedBandBoundsStayNil(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	parents := createTestParents(t, dbc)

	obj := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "plain", Title: "Plain",
	}
	_, err := dbc.NewInsert().Model(obj).Exec(ctx)
	require.NoError(t, err)

	got, err := repositories.NewObjectiveRepository(dbc).GetByID(ctx, obj.ID)
	require.NoError(t, err)
	assert.Nil(t, got.ChildrenMin)
	assert.Nil(t, got.ChildrenMax)
}

// --- Tree invariants ---

// parent_id has no foreign key, so the repository is the only thing standing
// between a mistyped id and a subtree no walk from the root can reach.
func TestObjectiveRepository_CreateTx_RejectsAnUnknownParent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).CreateTx(ctx, tx, &models.Objective{
		QuestID: fixture.questID, ParentID: "no-such-objective", Slug: "stray", Title: "Stray",
	})
	require.ErrorIs(t, err, repositories.ErrParentNotInQuest)
}

func TestObjectiveRepository_CreateTx_RejectsACrossQuestParent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	mine := insertTree(t, dbc)
	theirs := insertTree(t, dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).CreateTx(ctx, tx, &models.Objective{
		QuestID: mine.questID, ParentID: theirs.root.ID, Slug: "stray", Title: "Stray",
	})
	require.ErrorIs(t, err, repositories.ErrParentNotInQuest)
}

// A new child goes last, so positions stay dense without the caller having to
// read the siblings first.
func TestObjectiveRepository_CreateTx_AppendsToTheEnd(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)
	repo := repositories.NewObjectiveRepository(dbc)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	// Position on the model is ignored: placement belongs to Reposition.
	require.NoError(t, repo.CreateTx(ctx, tx, &models.Objective{
		QuestID: fixture.questID, ParentID: fixture.root.ID,
		Position: 0, Slug: "gamma", Title: "gamma",
	}))
	require.NoError(t, tx.Commit())

	children, err := repo.FindChildren(ctx, fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, slugsOf(children))
	assert.Equal(t, 2, children[2].Position)
}

func TestObjectiveRepository_FindRoot_ReportsAmbiguity(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)

	// Written directly, the way rows arrive before any tree is established.
	appendChild(t, dbc, fixture.questID, "", "second-root", 0)

	_, err := repositories.NewObjectiveRepository(dbc).FindRoot(ctx, fixture.questID)
	require.ErrorIs(t, err, repositories.ErrAmbiguousRootObjective)
}

func TestObjectiveRepository_FindRoot_ReportsAbsence(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	parents := createTestParents(t, dbc)

	_, err := repositories.NewObjectiveRepository(dbc).FindRoot(context.Background(), parents.QuestID)
	require.ErrorIs(t, err, repositories.ErrNoRootObjective)
}

// An objective with no parent is unattached, which is true of the root and of a
// row not yet placed. Only the root is immovable, and it is the root because it
// is the quest's only unattached row: attaching one of several builds the tree.
func TestObjectiveRepository_Reposition_AttachesAnUnplacedRow(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)
	repo := repositories.NewObjectiveRepository(dbc)

	unplaced := appendChild(t, dbc, fixture.questID, "", "unplaced", 0)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repo.Reposition(ctx, tx, unplaced.ID, fixture.root.ID, 0))
	require.NoError(t, tx.Commit())

	children, err := repo.FindChildren(ctx, fixture.questID, fixture.root.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"unplaced", "alpha", "beta"}, slugsOf(children))
}

// A parent that is itself stranded cannot host anything: hanging rows off it
// strands them too. The walk up from it never reaches an unattached row.
func TestObjectiveRepository_Reposition_RejectsAStrandedParent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	fixture := insertTree(t, dbc)

	stranded := appendChild(t, dbc, fixture.questID, "vanished-parent", "stranded", 0)

	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repositories.NewObjectiveRepository(dbc).
		Reposition(ctx, tx, fixture.beta.ID, stranded.ID, 0)
	require.ErrorIs(t, err, repositories.ErrParentStranded)
}

// FindChildren is quest-scoped: a parent id that leaked in from another quest
// must not pull that quest's rows into this one's tree.
func TestObjectiveRepository_FindChildren_DoesNotCrossQuests(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	mine := insertTree(t, dbc)
	theirs := insertTree(t, dbc)

	children, err := repositories.NewObjectiveRepository(dbc).
		FindChildren(context.Background(), mine.questID, theirs.root.ID)
	require.NoError(t, err)
	assert.Empty(t, children)
}
