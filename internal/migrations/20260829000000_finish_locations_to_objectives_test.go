package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Reuses m20260827_setupDB, m20260827_seedQuest, and the
// m20260827_seedMarker/m20260827_seedLocation row stand-ins from the sibling
// migration's own test file (same package): they already build exactly the
// "one quest, one already-convertible location" fixture this migration's sweep
// half needs to exercise, and duplicating them would just drift out of sync
// over time.

// m20260829_setupDB builds on m20260827_setupDB, whose full auto-migrate now
// also runs THIS package's own 20260829010000_drop_location migration (an
// empty DB has nothing to sweep, so it's a no-op there, then drops Location
// outright). By the time m20260827_setupDB returns, locations/markers/
// check_ins/uploads.location_id/runs.must_scan_out are already gone, and
// m20260827_seedQuest's plain INSERTs into locations/markers would fail. This
// restores that schema via the drop migration's own down function before
// seeding, so tests can exercise the up path against real starting data.
//
// objectives.when_clause is restored the same way, but by hand rather than
// through 20260831000000's own down function: the migrations replayed below
// write to when_clause while the assertions read models.Objective, which needs
// the depends column that same down function would take away.
func m20260829_setupDB(t *testing.T) *bun.DB {
	t.Helper()
	dbc := m20260827_setupDBThrough(t, "20260902000000")
	ctx := context.Background()
	require.NoError(t, m20260829010000_down(ctx, dbc))
	_, err := dbc.ExecContext(ctx, `ALTER TABLE "objectives" ADD COLUMN "when_clause" TEXT`)
	require.NoError(t, err)
	return dbc
}

// m20260829_readTree returns the quest's game_structure column decoded as a
// JSON tree, so assertions can inspect location_ids/objective_ids without a
// model that still knows those keys.
func m20260829_readTree(t *testing.T, dbc *bun.DB, questID string) map[string]any {
	t.Helper()
	ctx := context.Background()
	var raw string
	require.NoError(t, dbc.NewRaw("SELECT game_structure FROM quests WHERE id = ?", questID).Scan(ctx, &raw))
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &tree))
	return tree
}

func m20260829_writeTree(t *testing.T, dbc *bun.DB, questID string, tree map[string]any) {
	t.Helper()
	raw, err := json.Marshal(tree)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = dbc.NewRaw("UPDATE quests SET game_structure = ? WHERE id = ?", string(raw), questID).Exec(ctx)
	require.NoError(t, err)
}

func TestFinishMigration_StripsLocationIDsForAlreadyConvertedLocation(t *testing.T) {
	dbc := m20260829_setupDB(t)
	ctx := context.Background()
	questID, _ := m20260827_seedQuest(t, dbc)

	// Mimics 20260827 having already run.
	require.NoError(t, m20260827_up(ctx, dbc))

	var before []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&before).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, before, 1)

	require.NoError(t, m20260829_up(ctx, dbc))

	var after []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&after).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, after, 1, "no duplicate objective created for an already-converted location")
	assert.Equal(t, before[0].ID, after[0].ID)

	tree := m20260829_readTree(t, dbc, questID)
	assert.Empty(t, tree["location_ids"], "location_ids stripped once its objective exists")
	assert.Equal(t, []any{after[0].ID}, tree["objective_ids"], "objective_ids untouched")
}

func TestFinishMigration_SweepsStrandedMarkerlessLocationOnceItGetsAMarker(t *testing.T) {
	dbc := m20260829_setupDB(t)
	ctx := context.Background()
	questID, _ := m20260827_seedQuest(t, dbc)

	// A straggler: no marker, so 20260827's own run (mimicked here) skips it.
	strandedLoc := &m20260827_seedLocation{
		ID:      gofakeit.UUID(),
		QuestID: questID,
		Name:    "Stranded",
		Slug:    "stranded",
	}
	conn, err := dbc.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF")
	require.NoError(t, err)
	_, err = conn.NewInsert().Model(strandedLoc).Exec(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	tree := m20260829_readTree(t, dbc, questID)
	locIDs, ok := tree["location_ids"].([]any)
	require.True(t, ok, "seed tree should have location_ids")
	tree["location_ids"] = append(locIDs, strandedLoc.ID)
	m20260829_writeTree(t, dbc, questID, tree)

	require.NoError(t, m20260827_up(ctx, dbc))

	// 20260829 must fail loudly: strandedLoc still has no marker, and letting
	// the next migration drop Location would destroy it.
	err = m20260829_up(ctx, dbc)
	require.Error(t, err, "must not silently succeed while a location is still unconvertible")
	assert.Contains(t, err.Error(), strandedLoc.ID)

	tree = m20260829_readTree(t, dbc, questID)
	assert.Contains(
		t, tree["location_ids"], strandedLoc.ID,
		"unconverted location's ID must stay in the tree, not be silently dropped",
	)

	// Now the marker shows up.
	marker := &m20260827_seedMarker{Code: gofakeit.LetterN(5), Name: "New Sign"}
	_, err = dbc.NewInsert().Model(marker).Exec(ctx)
	require.NoError(t, err)
	strandedLoc.MarkerID = marker.Code
	_, err = dbc.NewUpdate().Model(strandedLoc).Column("marker_id").WherePK().Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, m20260829_up(ctx, dbc), "must succeed once every location is convertible")

	var objectives []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectives, 2, "both the originally-seeded and the stranded location are now objectives")

	tree = m20260829_readTree(t, dbc, questID)
	assert.Empty(t, tree["location_ids"], "location_ids fully stripped once everything converts")
}

func TestFinishMigration_NoLocations_NoOp(t *testing.T) {
	dbc := m20260829_setupDB(t)
	ctx := context.Background()

	require.NoError(t, m20260829_up(ctx, dbc), "must not error on a DB with no locations at all")
}

func TestFinishMigration_DownIsNoOp(t *testing.T) {
	dbc := m20260829_setupDB(t)
	ctx := context.Background()
	questID, _ := m20260827_seedQuest(t, dbc)

	require.NoError(t, m20260827_up(ctx, dbc))
	require.NoError(t, m20260829_up(ctx, dbc))

	before := m20260829_readTree(t, dbc, questID)

	require.NoError(t, m20260829_down(ctx, dbc))

	after := m20260829_readTree(t, dbc, questID)
	assert.Equal(t, before, after, "documented no-op: nothing changes on down")
}

func TestDropLocationMigration_DropsTablesOnceConversionComplete(t *testing.T) {
	dbc := m20260829_setupDB(t)
	ctx := context.Background()
	questID, _ := m20260827_seedQuest(t, dbc)

	require.NoError(t, m20260827_up(ctx, dbc))
	require.NoError(t, m20260829_up(ctx, dbc))

	var objectivesBefore []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectivesBefore).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectivesBefore, 1)
	var blocksBefore []models.Block
	require.NoError(t, dbc.NewSelect().Model(&blocksBefore).Where("owner_id = ?", objectivesBefore[0].ID).Scan(ctx))
	require.Len(t, blocksBefore, 4, "3 moved blocks + 1 synthetic scan block")

	require.NoError(t, m20260829010000_up(ctx, dbc))

	assert.False(t, tableExists(ctx, dbc, "locations"), "locations table dropped")
	assert.False(t, tableExists(ctx, dbc, "markers"), "markers table dropped")
	assert.False(t, tableExists(ctx, dbc, "check_ins"), "check_ins table dropped")
	assert.False(t, columnExists(ctx, dbc, "runs", "must_scan_out"), "must_scan_out column dropped")
	assert.False(t, columnExists(ctx, dbc, "uploads", "location_id"), "uploads.location_id column dropped")
	assert.False(t, columnExists(ctx, dbc, "quest_settings", "must_check_out"), "must_check_out column dropped")

	// The converted content survived the table drop untouched.
	var objectivesAfter []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectivesAfter).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectivesAfter, 1)
	assert.Equal(t, objectivesBefore[0].ID, objectivesAfter[0].ID)
	assert.Equal(t, objectivesBefore[0].Title, objectivesAfter[0].Title)

	var blocksAfter []models.Block
	require.NoError(t, dbc.NewSelect().Model(&blocksAfter).Where("owner_id = ?", objectivesAfter[0].ID).Scan(ctx))
	assert.Len(t, blocksAfter, 4, "objective's blocks survive the drop untouched")
}

func TestDropLocationMigration_NoLocationsTable_NoOp(t *testing.T) {
	// Deliberately m20260827_setupDB, not the wrapper: the full auto-migrate it
	// runs already drops Location (this package's own migration is part of that
	// chain), which is exactly the precondition this test wants to exercise.
	dbc := m20260827_setupDBThrough(t, "20260902000000")
	ctx := context.Background()

	require.NoError(t, m20260829010000_up(ctx, dbc))
	require.NoError(t, m20260829010000_up(ctx, dbc), "must be safe to run again once locations is already gone")
}

func TestDropLocationMigration_DownRestoresSchema(t *testing.T) {
	dbc := m20260829_setupDB(t)
	ctx := context.Background()
	_, _ = m20260827_seedQuest(t, dbc)

	require.NoError(t, m20260829_up(ctx, dbc))
	require.NoError(t, m20260829010000_up(ctx, dbc))

	require.NoError(t, m20260829010000_down(ctx, dbc))

	assert.True(t, tableExists(ctx, dbc, "locations"))
	assert.True(t, tableExists(ctx, dbc, "markers"))
	assert.True(t, tableExists(ctx, dbc, "check_ins"))
	assert.True(t, columnExists(ctx, dbc, "runs", "must_scan_out"))
	assert.True(t, columnExists(ctx, dbc, "uploads", "location_id"))
	assert.True(t, columnExists(ctx, dbc, "quest_settings", "must_check_out"))

	// Schema only: down does not resurrect dropped rows.
	count, err := dbc.NewSelect().Table("locations").Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "down restores empty tables, not the original data")
}
