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

// m20260827_seedQuest inserts a user, quest, marker, location (with an
// interactive content block, a non-interactive content block, and a
// navigation block), and returns the quest ID.
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
	marker := &models.Marker{Code: markerCode, Name: "Old Sign"}
	_, err = dbc.NewInsert().Model(marker).Exec(ctx)
	require.NoError(t, err)

	loc := &models.Location{
		ID:       gofakeit.UUID(),
		QuestID:  quest.ID,
		Name:     "Old Location",
		Slug:     "old-location",
		MarkerID: markerCode,
		Points:   15,
	}
	_, err = dbc.NewInsert().Model(loc).Exec(ctx)
	require.NoError(t, err)

	quest.GameStructure = models.GameStructure{
		ID:             gofakeit.UUID(),
		IsRoot:         true,
		Routing:        game.RouteStrategyFreeRoam,
		CompletionType: game.CompletionAll,
		LocationIDs:    []string{loc.ID},
		SubGroups:      []models.GameStructure{},
	}
	_, err = dbc.NewUpdate().Model(quest).Column("game_structure").WherePK().Exec(ctx)
	require.NoError(t, err)

	interactiveBlock := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: loc.ID,
		Type:    "quiz",
		Context: game.ContextLocationContent,
		Data:    json.RawMessage(`{"prompt":"What is 2+2?"}`),
	}
	_, err = dbc.NewInsert().Model(interactiveBlock).Exec(ctx)
	require.NoError(t, err)

	contentBlock := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: loc.ID,
		Type:    "text",
		Context: game.ContextLocationContent,
		Data:    json.RawMessage(`{"content":"Welcome!"}`),
	}
	_, err = dbc.NewInsert().Model(contentBlock).Exec(ctx)
	require.NoError(t, err)

	navBlock := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: loc.ID,
		Type:    "clue",
		Context: game.ContextNavigation,
		Data:    json.RawMessage(`{"clue":"Look near the fountain"}`),
	}
	_, err = dbc.NewInsert().Model(navBlock).Exec(ctx)
	require.NoError(t, err)

	return quest.ID, loc.ID
}

func TestLocationsToObjectivesMigration_ConvertsLocationCorrectly(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	questID, locationID := m20260827_seedQuest(t, dbc)

	require.NoError(t, m20260827_up(ctx, dbc))

	var objectives []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectives, 1, "exactly one objective created for the one location")
	obj := objectives[0]
	assert.Equal(t, "old-location", obj.Slug)
	assert.Equal(t, "Old Location", obj.Title)

	var blocks []models.Block
	require.NoError(t, dbc.NewSelect().Model(&blocks).Where("owner_id = ?", obj.ID).Scan(ctx))
	require.Len(t, blocks, 4, "3 moved blocks + 1 new scan block")

	byType := make(map[string]models.Block, len(blocks))
	for _, b := range blocks {
		byType[b.Type] = b
	}

	quiz, ok := byType["quiz"]
	require.True(t, ok, "interactive content block must be present")
	assert.Equal(t, game.ContextObjectiveProof, quiz.Context, "interactive content block joins proof")

	text, ok := byType["text"]
	require.True(t, ok, "non-interactive content block must be present")
	assert.Equal(t, game.ContextObjectiveReveal, text.Context, "non-interactive content block becomes reveal")

	clue, ok := byType["clue"]
	require.True(t, ok, "navigation block must be present")
	assert.Equal(t, game.ContextObjectiveProof, clue.Context, "navigation block folds into proof")

	scan, ok := byType["scan"]
	require.True(t, ok, "a new scan block must be created")
	assert.Equal(t, game.ContextObjectiveProof, scan.Context)
	assert.Equal(t, 15, scan.Points, "location points move onto the scan block")
	var scanData struct {
		Codes []struct {
			Value    string `json:"value"`
			Generate bool   `json:"generate"`
		} `json:"codes"`
	}
	require.NoError(t, json.Unmarshal(scan.Data, &scanData))
	require.Len(t, scanData.Codes, 1)

	var loc models.Location
	require.NoError(t, dbc.NewSelect().Model(&loc).Where("id = ?", locationID).Scan(ctx))
	assert.Equal(t, scanData.Codes[0].Value, loc.MarkerID, "scan code is the old marker code")
	assert.False(t, scanData.Codes[0].Generate, "the code already exists physically, don't ask Rapua to mint a new one")

	// Location and its marker are untouched.
	assert.Equal(t, "Old Location", loc.Name)
	var marker models.Marker
	require.NoError(t, dbc.NewSelect().Model(&marker).Where("code = ?", loc.MarkerID).Scan(ctx))
	assert.Equal(t, "Old Sign", marker.Name)

	// GameStructure now references the objective alongside the untouched location.
	var quest models.Quest
	require.NoError(t, dbc.NewSelect().Model(&quest).Where("id = ?", questID).Scan(ctx))
	assert.Equal(t, []string{locationID}, quest.GameStructure.LocationIDs, "location_ids untouched")
	assert.Equal(t, []string{obj.ID}, quest.GameStructure.ObjectiveIDs)
}

func TestLocationsToObjectivesMigration_Idempotent(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	questID, _ := m20260827_seedQuest(t, dbc)

	require.NoError(t, m20260827_up(ctx, dbc))
	require.NoError(t, m20260827_up(ctx, dbc))

	var objectives []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	assert.Len(t, objectives, 1, "running the migration twice must not duplicate the objective")
}

func TestLocationsToObjectivesMigration_DownReverts(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	questID, locationID := m20260827_seedQuest(t, dbc)

	require.NoError(t, m20260827_up(ctx, dbc))
	require.NoError(t, m20260827_down(ctx, dbc))

	var objectives []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	assert.Empty(t, objectives, "objectives deleted on down")

	var quest models.Quest
	require.NoError(t, dbc.NewSelect().Model(&quest).Where("id = ?", questID).Scan(ctx))
	assert.Empty(t, quest.GameStructure.ObjectiveIDs, "objective_ids stripped on down")
	assert.Equal(t, []string{locationID}, quest.GameStructure.LocationIDs, "location_ids still untouched")

	var loc models.Location
	require.NoError(t, dbc.NewSelect().Model(&loc).Where("id = ?", locationID).Scan(ctx))
	assert.Equal(t, "Old Location", loc.Name, "location was never touched by up, so nothing to revert")

	var blocks []models.Block
	require.NoError(t, dbc.NewSelect().Model(&blocks).Where("owner_id = ?", locationID).Scan(ctx))
	require.Len(t, blocks, 3, "the original 3 location blocks are back with their original owner")
	for _, b := range blocks {
		// Disclosed, honest limitation: down cannot tell whether a proof-context
		// block originally came from Content or Navigation (up moves both there),
		// so it restores everything to location_content. No content is lost:
		// the "clue" block here really was Navigation before up ran, and still
		// ends up back as location_content, not navigation. See the doc comment
		// on m20260827_down.
		assert.Equal(t, game.ContextLocationContent, b.Context, "type=%s", b.Type)
	}

	var scanLeftover []models.Block
	require.NoError(t, dbc.NewSelect().Model(&scanLeftover).Where("type = ?", "scan").Scan(ctx))
	assert.Empty(t, scanLeftover, "the synthetic scan block is deleted, not restored anywhere")
}

func TestLocationsToObjectivesMigration_MarkerlessLocationSkippedNotStuck(t *testing.T) {
	dbc := m20260827_setupDB(t)
	ctx := context.Background()
	questID, _ := m20260827_seedQuest(t, dbc)

	// A second location in the same quest, with no marker assigned yet.
	// locations.marker_id is NOT NULL + FK'd to markers.code in the real schema,
	// so this cannot happen through the app's own insert paths: the FK bypass
	// below only exists to exercise the migration's defensive skip.
	markerlessLoc := &models.Location{
		ID:      gofakeit.UUID(),
		QuestID: questID,
		Name:    "No Marker Yet",
		Slug:    "no-marker-yet",
		Points:  5,
	}
	conn, err := dbc.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF")
	require.NoError(t, err)
	_, err = conn.NewInsert().Model(markerlessLoc).Exec(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	var quest models.Quest
	require.NoError(t, dbc.NewSelect().Model(&quest).Where("id = ?", questID).Scan(ctx))
	quest.GameStructure.LocationIDs = append(quest.GameStructure.LocationIDs, markerlessLoc.ID)
	_, err = dbc.NewUpdate().Model(&quest).Column("game_structure").WherePK().Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, m20260827_up(ctx, dbc))

	var objectives []models.Objective
	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectives, 1, "only the markered location converts")
	assert.Equal(t, "old-location", objectives[0].Slug)

	var markerlessBlocks []models.Block
	require.NoError(t, dbc.NewSelect().Model(&markerlessBlocks).
		Where("owner_id = ?", markerlessLoc.ID).Scan(ctx))
	assert.Empty(t, markerlessBlocks, "no scan block created for the markerless location")

	// Re-running must not get stuck skipping the whole quest just because it
	// already holds an objective: the markered location must not be
	// re-converted (still exactly 1 objective), and the markerless one is
	// still correctly left unconverted.
	require.NoError(t, m20260827_up(ctx, dbc))

	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectives, 1, "re-run does not duplicate the already-converted location")

	// Now the marker shows up, mimicking an admin finally assigning one.
	marker := &models.Marker{Code: gofakeit.LetterN(5), Name: "New Sign"}
	_, err = dbc.NewInsert().Model(marker).Exec(ctx)
	require.NoError(t, err)
	markerlessLoc.MarkerID = marker.Code
	_, err = dbc.NewUpdate().Model(markerlessLoc).Column("marker_id").WherePK().Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, m20260827_up(ctx, dbc))

	require.NoError(t, dbc.NewSelect().Model(&objectives).Where("quest_id = ?", questID).Scan(ctx))
	require.Len(t, objectives, 2, "the now-markered location converts on the next run, once it has a marker")
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
