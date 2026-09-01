package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// m20260902000000_seed writes a quest carrying the given blob, plus one
// objective row per id named in it, the way the database looked before the
// structure moved out of the blob.
func m20260902000000_seed(
	t *testing.T, dbc *bun.DB, questName string, root m20260902000000_group, objectiveIDs ...string,
) string {
	t.Helper()
	ctx := context.Background()
	questID := gofakeit.UUID()

	blob, err := json.Marshal(root)
	require.NoError(t, err)
	_, err = dbc.ExecContext(ctx,
		`INSERT INTO "quests" ("id", "name", "game_structure") VALUES (?, ?, ?)`,
		questID, questName, string(blob))
	require.NoError(t, err)

	for i, id := range objectiveIDs {
		_, err = dbc.ExecContext(ctx,
			`INSERT INTO "objectives" ("id", "quest_id", "slug", "title") VALUES (?, ?, ?, ?)`,
			id, questID, "obj-"+id[:8], gofakeit.Word()+string(rune('A'+i)))
		require.NoError(t, err)
	}
	return questID
}

type m20260902000000_row struct {
	ID       string
	ParentID string
	Position int
	Slug     string
	Title    string
}

func m20260902000000_read(t *testing.T, dbc *bun.DB, questID string) []m20260902000000_row {
	t.Helper()
	rows, err := dbc.QueryContext(context.Background(),
		`SELECT "id", COALESCE("parent_id", ''), "position", "slug", "title" FROM "objectives"`+
			` WHERE "quest_id" = ? ORDER BY "parent_id", "position"`, questID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []m20260902000000_row
	for rows.Next() {
		var r m20260902000000_row
		require.NoError(t, rows.Scan(&r.ID, &r.ParentID, &r.Position, &r.Slug, &r.Title))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func m20260902000000_childrenOf(rows []m20260902000000_row, parentID string) []m20260902000000_row {
	var out []m20260902000000_row
	for _, r := range rows {
		if r.ParentID == parentID {
			out = append(out, r)
		}
	}
	return out
}

func m20260902000000_root(t *testing.T, rows []m20260902000000_row) m20260902000000_row {
	t.Helper()
	roots := m20260902000000_childrenOf(rows, "")
	require.Len(t, roots, 1, "a quest has exactly one objective with no parent")
	return roots[0]
}

// A group becomes an objective with children, and within a node the blob's
// objectives come before its subgroups.
func TestStructureBlobToRows_BuildsTheTree(t *testing.T) {
	dbc := m20260827_setupDB(t)
	objA, objB, nested := gofakeit.UUID(), gofakeit.UUID(), gofakeit.UUID()

	questID := m20260902000000_seed(t, dbc, "Harbour Hunt", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam", CompletionType: "all",
		ObjectiveIDs: []string{objA, objB},
		SubGroups: []m20260902000000_group{{
			ID: gofakeit.UUID(), Name: "East Wing", Color: "primary",
			Routing: "ordered", CompletionType: "all",
			ObjectiveIDs: []string{nested},
		}},
	}, objA, objB, nested)

	require.NoError(t, m20260902000000_up(context.Background(), dbc))
	rows := m20260902000000_read(t, dbc, questID)

	root := m20260902000000_root(t, rows)
	assert.Equal(t, "Harbour Hunt", root.Title, "the root is named for its quest")

	children := m20260902000000_childrenOf(rows, root.ID)
	require.Len(t, children, 3)
	assert.Equal(t, []string{objA, objB}, []string{children[0].ID, children[1].ID},
		"objectives come before subgroups")
	assert.Equal(t, "East Wing", children[2].Title)
	assert.Equal(t, []int{0, 1, 2}, []int{children[0].Position, children[1].Position, children[2].Position})

	assert.Equal(t, []string{nested},
		[]string{m20260902000000_childrenOf(rows, children[2].ID)[0].ID},
		"a subgroup keeps its own objectives")
}

// Duplicating a quest copied the blob verbatim, so the same group id appears in
// several quests. An objective id is unique across all of them, so a section
// cannot reuse one.
func TestStructureBlobToRows_RepeatedBlobIDsDoNotCollide(t *testing.T) {
	dbc := m20260827_setupDB(t)
	sharedID := gofakeit.UUID()

	blob := m20260902000000_group{ID: sharedID, Routing: "free_roam", CompletionType: "all"}
	firstQuest := m20260902000000_seed(t, dbc, "Original", blob)
	secondQuest := m20260902000000_seed(t, dbc, "Duplicate", blob)

	require.NoError(t, m20260902000000_up(context.Background(), dbc))

	firstRoot := m20260902000000_root(t, m20260902000000_read(t, dbc, firstQuest))
	secondRoot := m20260902000000_root(t, m20260902000000_read(t, dbc, secondQuest))
	assert.NotEqual(t, firstRoot.ID, secondRoot.ID, "each quest gets its own section rows")
}

func TestStructureBlobToRows_Band(t *testing.T) {
	tests := []struct {
		name            string
		completion      string
		minimumRequired int
		autoAdvance     bool
		wantMin         any
		wantMax         any
	}{
		{name: "completion all requires every child", completion: "all"},
		{
			name:       "minimum with auto-advance auto-completes at the minimum",
			completion: "minimum", minimumRequired: 1, autoAdvance: true,
			wantMin: int64(1), wantMax: int64(1),
		},
		{
			name:       "minimum without auto-advance leaves the player to finish",
			completion: "minimum", minimumRequired: 2,
			wantMin: int64(2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbc := m20260827_setupDB(t)
			objID := gofakeit.UUID()
			questID := m20260902000000_seed(t, dbc, "Banded", m20260902000000_group{
				ID: gofakeit.UUID(), Routing: "free_roam",
				CompletionType: tt.completion, MinimumRequired: tt.minimumRequired,
				AutoAdvance:  tt.autoAdvance,
				ObjectiveIDs: []string{objID},
			}, objID)

			require.NoError(t, m20260902000000_up(context.Background(), dbc))

			var gotMin, gotMax any
			require.NoError(t, dbc.QueryRowContext(context.Background(),
				`SELECT "children_min", "children_max" FROM "objectives"`+
					` WHERE "quest_id" = ? AND ("parent_id" IS NULL OR "parent_id" = '')`, questID,
			).Scan(&gotMin, &gotMax))
			assert.Equal(t, tt.wantMin, gotMin)
			assert.Equal(t, tt.wantMax, gotMax)
		})
	}
}

// Group names repeat within a quest, and the objectives table holds a unique
// index on (quest_id, slug).
func TestStructureBlobToRows_SuffixesRepeatedSlugs(t *testing.T) {
	dbc := m20260827_setupDB(t)
	sameName := m20260902000000_group{
		ID: gofakeit.UUID(), Name: "Locations", Routing: "free_roam", CompletionType: "all",
	}

	questID := m20260902000000_seed(t, dbc, "Repeats", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam", CompletionType: "all",
		SubGroups: []m20260902000000_group{sameName, sameName},
	})

	require.NoError(t, m20260902000000_up(context.Background(), dbc))
	rows := m20260902000000_read(t, dbc, questID)

	slugs := map[string]bool{}
	for _, r := range rows {
		assert.False(t, slugs[r.Slug], "slug %q used twice", r.Slug)
		slugs[r.Slug] = true
	}
	assert.True(t, slugs["locations"] && slugs["locations-2"])
}

// An id left behind by a deleted objective places nothing, and an objective the
// blob never mentioned still needs somewhere to live: unplaced, it would read
// as a second root.
func TestStructureBlobToRows_SkipsStaleIDsAndAdoptsOrphans(t *testing.T) {
	dbc := m20260827_setupDB(t)
	known, orphan := gofakeit.UUID(), gofakeit.UUID()

	questID := m20260902000000_seed(t, dbc, "Strays", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam", CompletionType: "all",
		ObjectiveIDs: []string{known, gofakeit.UUID()},
	}, known, orphan)

	require.NoError(t, m20260902000000_up(context.Background(), dbc))
	rows := m20260902000000_read(t, dbc, questID)

	root := m20260902000000_root(t, rows)
	children := m20260902000000_childrenOf(rows, root.ID)
	require.Len(t, children, 2, "the stale id placed nothing, the orphan was adopted")
	assert.Equal(t, known, children[0].ID)
	assert.Equal(t, orphan, children[1].ID)
}

// A rerun after a partial failure must find its own work rather than building
// the tree a second time.
func TestStructureBlobToRows_RerunIsANoOp(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	objID := gofakeit.UUID()

	questID := m20260902000000_seed(t, dbc, "Twice", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam", CompletionType: "all",
		ObjectiveIDs: []string{objID},
	}, objID)

	require.NoError(t, m20260902000000_up(ctx, dbc))
	before := m20260902000000_read(t, dbc, questID)
	require.NoError(t, m20260902000000_up(ctx, dbc))

	assert.Equal(t, before, m20260902000000_read(t, dbc, questID))
}

// A blob new enough to carry the band carries it verbatim, and only it can
// express a range. Deriving from the completion trio instead would narrow
// [1,3] to [1, omitted] and lose the upper bound the author wrote.
func TestStructureBlobToRows_PrefersTheVerbatimBand(t *testing.T) {
	dbc := m20260827_setupDB(t)
	minChildren, maxChildren := 1, 3

	questID := m20260902000000_seed(t, dbc, "Ranged", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam",
		ChildrenMin: &minChildren, ChildrenMax: &maxChildren,
		// The trio says something narrower, and must not win.
		CompletionType: "minimum", MinimumRequired: 1,
	})

	require.NoError(t, m20260902000000_up(context.Background(), dbc))

	var gotMin, gotMax any
	require.NoError(t, dbc.QueryRowContext(context.Background(),
		`SELECT "children_min", "children_max" FROM "objectives"`+
			` WHERE "quest_id" = ? AND ("parent_id" IS NULL OR "parent_id" = '')`, questID,
	).Scan(&gotMin, &gotMax))
	assert.Equal(t, int64(1), gotMin)
	assert.Equal(t, int64(3), gotMax, "the upper bound survives")
}

// Slug, finish label and depends are stored on the blob too. A section's slug
// in particular is what a depends list names it by, so re-minting one from the
// title would break every gate pointing at it.
func TestStructureBlobToRows_CarriesTheStoredFields(t *testing.T) {
	dbc := m20260827_setupDB(t)

	questID := m20260902000000_seed(t, dbc, "Carried", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam", CompletionType: "all",
		SubGroups: []m20260902000000_group{{
			ID: gofakeit.UUID(), Name: "Renamed Since", Slug: "east-wing",
			Routing: "free_roam", CompletionType: "all",
			FinishLabel: "Leave the wing",
			Depends:     []byte(`["found_key"]`),
		}},
	})

	require.NoError(t, m20260902000000_up(context.Background(), dbc))

	var slug, finishLabel, depends string
	require.NoError(t, dbc.QueryRowContext(context.Background(),
		`SELECT "slug", "finish_label", "depends" FROM "objectives"`+
			` WHERE "quest_id" = ? AND "parent_id" != '' AND "parent_id" IS NOT NULL`, questID,
	).Scan(&slug, &finishLabel, &depends))

	assert.Equal(t, "east-wing", slug, "a stored slug survives a rename")
	assert.Equal(t, "Leave the wing", finishLabel)
	assert.JSONEq(t, `["found_key"]`, depends)
}

// A stale id places nothing, so numbering the objectives that follow it from
// the blob's declared count would leave a hole where it would have sat.
func TestStructureBlobToRows_PositionsStayDense(t *testing.T) {
	dbc := m20260827_setupDB(t)
	known, orphan := gofakeit.UUID(), gofakeit.UUID()

	questID := m20260902000000_seed(t, dbc, "Dense", m20260902000000_group{
		ID: gofakeit.UUID(), Routing: "free_roam", CompletionType: "all",
		ObjectiveIDs: []string{gofakeit.UUID(), known, gofakeit.UUID()},
	}, known, orphan)

	require.NoError(t, m20260902000000_up(context.Background(), dbc))
	rows := m20260902000000_read(t, dbc, questID)

	children := m20260902000000_childrenOf(rows, m20260902000000_root(t, rows).ID)
	require.Len(t, children, 2)
	assert.Equal(t, []int{0, 1}, []int{children[0].Position, children[1].Position})
}
