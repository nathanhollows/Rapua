package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

func m20260827_setupDB(t *testing.T) *bun.DB {
	t.Helper()
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	dbc := db.MustOpen(slog.New(slog.NewTextHandler(m20260827_testWriter{t}, nil)))
	ctx := context.Background()

	migrator := migrate.NewMigrator(dbc, Migrations)
	require.NoError(t, migrator.Init(ctx))
	require.NoError(t, migrator.Lock(ctx))
	defer func() { require.NoError(t, migrator.Unlock(ctx)) }()
	_, err := migrator.Migrate(ctx)
	require.NoError(t, err)

	t.Cleanup(func() { dbc.Close() })
	return dbc
}

type m20260827_testWriter struct{ t *testing.T }

func (w m20260827_testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

// m20260827_seedMarker and m20260827_seedLocation are point-in-time stand-ins
// for the rows that models.Marker and models.Location used to express: the
// drop migration removes both types (and eventually both tables), so the seed
// fixture speaks the restored table schema directly.
type m20260827_seedMarker struct {
	bun.BaseModel `bun:"table:markers"`
	Code          string `bun:"code,pk"`
	Name          string `bun:"name"`
}

type m20260827_seedLocation struct {
	bun.BaseModel `bun:"table:locations"`
	ID            string `bun:"id,pk"`
	QuestID       string `bun:"quest_id"`
	Name          string `bun:"name"`
	Slug          string `bun:"slug"`
	MarkerID      string `bun:"marker_id"`
	Points        int    `bun:"points"`
}

// m20260827_seedQuest inserts a user, quest, marker, location (with an
// interactive content block, a non-interactive content block, and a
// navigation block), and returns the quest and location IDs. The
// game_structure column is written as raw JSON mirroring what
// models.GameStructure used to marshal, because that model no longer carries
// location_ids.
func m20260827_seedQuest(t *testing.T, dbc *bun.DB) (questID string, locationID string) {
	t.Helper()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	quest := &models.Quest{
		ID:     gofakeit.UUID(),
		Name:   "Migrate Me",
		UserID: user.ID,
	}
	_, err = dbc.NewInsert().Model(quest).Exec(ctx)
	require.NoError(t, err)

	markerCode := gofakeit.LetterN(5)
	marker := &m20260827_seedMarker{Code: markerCode, Name: "Old Sign"}
	_, err = dbc.NewInsert().Model(marker).Exec(ctx)
	require.NoError(t, err)

	loc := &m20260827_seedLocation{
		ID:       gofakeit.UUID(),
		QuestID:  quest.ID,
		Name:     "Old Location",
		Slug:     "old-location",
		MarkerID: markerCode,
		Points:   15,
	}
	_, err = dbc.NewInsert().Model(loc).Exec(ctx)
	require.NoError(t, err)

	tree := map[string]any{
		"id":              gofakeit.UUID(),
		"name":            "",
		"color":           "",
		"routing":         game.RouteStrategyFreeRoam,
		"completion_type": game.CompletionAll,
		"auto_advance":    false,
		"is_root":         true,
		"location_ids":    []string{loc.ID},
		"sub_groups":      []any{},
	}
	treeJSON, err := json.Marshal(tree)
	require.NoError(t, err)
	_, err = dbc.NewRaw("UPDATE quests SET game_structure = ? WHERE id = ?", string(treeJSON), quest.ID).Exec(ctx)
	require.NoError(t, err)

	interactiveBlock := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: loc.ID,
		Type:    "quiz",
		Context: game.BlockContext("location_content"),
		Data:    json.RawMessage(`{"prompt":"What is 2+2?"}`),
	}
	_, err = dbc.NewInsert().Model(interactiveBlock).Exec(ctx)
	require.NoError(t, err)

	contentBlock := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: loc.ID,
		Type:    "text",
		Context: game.BlockContext("location_content"),
		Data:    json.RawMessage(`{"content":"Welcome!"}`),
	}
	_, err = dbc.NewInsert().Model(contentBlock).Exec(ctx)
	require.NoError(t, err)

	navBlock := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: loc.ID,
		Type:    "clue",
		Context: game.BlockContext("navigation"),
		Data:    json.RawMessage(`{"clue":"Look near the fountain"}`),
	}
	_, err = dbc.NewInsert().Model(navBlock).Exec(ctx)
	require.NoError(t, err)

	return quest.ID, loc.ID
}

func TestAddObjectiveIDsToTree_AppendsAtEachNode(t *testing.T) {
	input := `{
		"location_ids": ["loc1", "loc2"],
		"sub_groups": [
			{
				"location_ids": ["loc3"],
				"sub_groups": []
			}
		]
	}`
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &tree))

	objIDByLocID := map[string]string{
		"loc1": "obj1",
		"loc2": "obj2",
		"loc3": "obj3",
	}

	changed := m20260827_addObjectiveIDsToTree(tree, objIDByLocID)
	assert.True(t, changed)

	rootObjIDs := tree["objective_ids"].([]any)
	assert.Equal(t, []any{"obj1", "obj2"}, rootObjIDs, "order preserved, matching location_ids order")

	subs := tree["sub_groups"].([]any)
	sub := subs[0].(map[string]any)
	assert.Equal(t, []any{"obj3"}, sub["objective_ids"])
}

func TestAddObjectiveIDsToTree_UnmappedLocationIDsSkipped(t *testing.T) {
	input := `{"location_ids": ["loc1", "loc2"], "sub_groups": []}`
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &tree))

	// Only loc1 has a mapping (loc2 might have been skipped due to a missing marker).
	changed := m20260827_addObjectiveIDsToTree(tree, map[string]string{"loc1": "obj1"})
	assert.True(t, changed)
	assert.Equal(t, []any{"obj1"}, tree["objective_ids"])
}

func TestAddObjectiveIDsToTree_NoMatches_NoChange(t *testing.T) {
	input := `{"location_ids": ["loc1"], "sub_groups": []}`
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &tree))

	changed := m20260827_addObjectiveIDsToTree(tree, map[string]string{"other-loc": "obj1"})
	assert.False(t, changed)
	_, has := tree["objective_ids"]
	assert.False(t, has, "no objective_ids key added when nothing matched")
}

func TestRemoveObjectiveIDsFromTree(t *testing.T) {
	input := `{
		"location_ids": ["loc1"],
		"objective_ids": ["obj1"],
		"sub_groups": [
			{"location_ids": [], "objective_ids": ["obj2"], "sub_groups": []}
		]
	}`
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &tree))

	changed := m20260827_removeObjectiveIDsFromTree(tree, map[string]bool{"obj1": true, "obj2": true})
	assert.True(t, changed)

	rootIDs, _ := tree["objective_ids"].([]any)
	assert.Empty(t, rootIDs)
	sub := tree["sub_groups"].([]any)[0].(map[string]any)
	subIDs, _ := sub["objective_ids"].([]any)
	assert.Empty(t, subIDs)
	// location_ids untouched
	assert.Equal(t, []any{"loc1"}, tree["location_ids"])
}

func TestRemoveObjectiveIDsFromTree_LeavesUnmatchedIDsInPlace(t *testing.T) {
	// obj2 was not created by this migration (e.g. real objective authoring);
	// removing obj1 must not touch it.
	input := `{"location_ids": [], "objective_ids": ["obj1", "obj2"], "sub_groups": []}`
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &tree))

	changed := m20260827_removeObjectiveIDsFromTree(tree, map[string]bool{"obj1": true})
	assert.True(t, changed)
	assert.Equal(t, []any{"obj2"}, tree["objective_ids"], "unmatched objective ID left in place")
}

func TestAddThenRemoveObjectiveIDs_RoundTrip(t *testing.T) {
	input := `{"location_ids": ["loc1"], "sub_groups": []}`
	var tree map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &tree))

	assert.True(t, m20260827_addObjectiveIDsToTree(tree, map[string]string{"loc1": "obj1"}))
	assert.Equal(t, []any{"obj1"}, tree["objective_ids"])

	assert.True(t, m20260827_removeObjectiveIDsFromTree(tree, map[string]bool{"obj1": true}))
	ids, _ := tree["objective_ids"].([]any)
	assert.Empty(t, ids)
	assert.Equal(t, []any{"loc1"}, tree["location_ids"], "location_ids was never touched")
}
