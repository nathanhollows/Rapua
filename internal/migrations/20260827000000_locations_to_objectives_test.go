package migrations //nolint:testpackage // testing unexported migration helpers

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// m20260827_setupDBThrough stops early, for seeding what a later migration
// drops.
func m20260827_setupDBThrough(t *testing.T, through string) *bun.DB {
	t.Helper()
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	dbc := db.MustOpen(slog.New(slog.NewTextHandler(m20260827_testWriter{t}, nil)))
	ctx := context.Background()

	set := Migrations
	if through != "" {
		set = migrate.NewMigrations()
		for _, m := range Migrations.Sorted() {
			if m.Name > through {
				continue
			}
			set.Add(m)
		}
	}

	migrator := migrate.NewMigrator(dbc, set)
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

func m20260827_setupDB(t *testing.T) *bun.DB {
	t.Helper()
	return m20260827_setupDBThrough(t, "")
}

func m20260827_setupPreConversion(t *testing.T) *bun.DB {
	t.Helper()
	return m20260827_setupDBThrough(t, "20260822000000")
}

func m20260827_exec(t *testing.T, dbc *bun.DB, query string, args ...any) {
	t.Helper()
	_, err := dbc.ExecContext(context.Background(), query, args...)
	require.NoError(t, err)
}

// m20260827_seedQuest leaves the blob NULL: a test writes one once it has the
// location ids.
func m20260827_seedQuest(t *testing.T, dbc *bun.DB, name string) string {
	t.Helper()
	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(context.Background())
	require.NoError(t, err)

	questID := gofakeit.UUID()
	m20260827_exec(t, dbc,
		`INSERT INTO "quests" ("id", "name", "user_id") VALUES (?, ?, ?)`, questID, name, user.ID)
	return questID
}

func m20260827_seedLocation(t *testing.T, dbc *bun.DB, questID, name, slug string, points int) (string, string) {
	t.Helper()
	code := gofakeit.LetterN(6)
	m20260827_exec(t, dbc, `INSERT INTO "markers" ("code", "name") VALUES (?, ?)`, code, name)
	id := gofakeit.UUID()
	m20260827_exec(t, dbc,
		`INSERT INTO "locations" ("id", "quest_id", "name", "slug", "marker_id", "points") VALUES (?, ?, ?, ?, ?, ?)`,
		id, questID, name, slug, code, points)
	return id, code
}

func m20260827_seedBlock(t *testing.T, dbc *bun.DB, ownerID, blockType, blockContext string) string {
	t.Helper()
	id := gofakeit.UUID()
	m20260827_exec(t, dbc,
		`INSERT INTO "blocks" ("id", "owner_id", "type", "context", "data", "ordering", "points")`+
			` VALUES (?, ?, ?, ?, '{}', 0, 0)`,
		id, ownerID, blockType, blockContext)
	return id
}

func m20260827_seedRun(t *testing.T, dbc *bun.DB, questID string) string {
	t.Helper()
	code := gofakeit.LetterN(4)
	m20260827_exec(t, dbc,
		`INSERT INTO "runs" ("id", "code", "quest_id", "has_started", "points") VALUES (?, ?, ?, TRUE, 0)`,
		gofakeit.UUID(), code, questID)
	return code
}

func m20260827_seedCheckIn(t *testing.T, dbc *bun.DB, runCode, questID, locationID string, completed bool) {
	t.Helper()
	m20260827_exec(t, dbc,
		`INSERT INTO "check_ins" ("run_code", "quest_id", "location_id", "time_in", "blocks_completed")`+
			` VALUES (?, ?, ?, '2026-03-01 10:00:00', ?)`,
		runCode, questID, locationID, completed)
}

type m20260827_row struct {
	ID          string
	ParentID    string
	Position    int
	Slug        string
	Title       string
	Routing     string
	ChildrenMin *int
	ChildrenMax *int
}

func m20260827_rows(t *testing.T, dbc *bun.DB, questID string) []m20260827_row {
	t.Helper()
	rows, err := dbc.QueryContext(context.Background(),
		`SELECT "id", COALESCE("parent_id", ''), "position", "slug", "title", COALESCE("routing", ''),`+
			` "children_min", "children_max" FROM "objectives" WHERE "quest_id" = ? ORDER BY "parent_id", "position"`,
		questID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []m20260827_row
	for rows.Next() {
		var r m20260827_row
		require.NoError(t, rows.Scan(&r.ID, &r.ParentID, &r.Position, &r.Slug, &r.Title, &r.Routing,
			&r.ChildrenMin, &r.ChildrenMax))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func m20260827_childrenOf(rows []m20260827_row, parentID string) []m20260827_row {
	var out []m20260827_row
	for _, r := range rows {
		if r.ParentID == parentID {
			out = append(out, r)
		}
	}
	return out
}

func m20260827_root(t *testing.T, rows []m20260827_row) m20260827_row {
	t.Helper()
	roots := m20260827_childrenOf(rows, "")
	require.Len(t, roots, 1, "a quest has exactly one objective with no parent")
	return roots[0]
}

func m20260827_titles(rows []m20260827_row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Title)
	}
	return out
}

func m20260827_count(t *testing.T, dbc *bun.DB, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, dbc.QueryRowContext(context.Background(), query, args...).Scan(&n))
	return n
}

func TestLocationsToObjectives_BuildsTheTreeFromTheBlob(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Harbour Walk")
	loose, _ := m20260827_seedLocation(t, dbc, questID, "Ferry Terminal", "ferry-terminal", 5)
	first, _ := m20260827_seedLocation(t, dbc, questID, "Lighthouse", "lighthouse", 10)
	second, _ := m20260827_seedLocation(t, dbc, questID, "Boat Shed", "boat-shed", 10)
	nested, _ := m20260827_seedLocation(t, dbc, questID, "Crane", "crane", 10)

	blob := map[string]any{
		"is_root": true, "routing": "free_roam", "completion_type": "all",
		"location_ids": []string{loose},
		"sub_groups": []any{map[string]any{
			"name": "The Wharf", "routing": "randomised", "max_next": 1,
			"completion_type": "minimum", "minimum_required": 1, "auto_advance": true,
			"location_ids": []string{first, second},
			"sub_groups": []any{map[string]any{
				"name": "Cargo Yard", "routing": "ordered", "completion_type": "all",
				"location_ids": []string{nested},
			}},
		}},
	}
	raw, err := json.Marshal(blob)
	require.NoError(t, err)
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = ? WHERE "id" = ?`, string(raw), questID)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	root := m20260827_root(t, rows)
	assert.Equal(t, "Harbour Walk", root.Title, "the root is named after the quest")
	assert.Equal(t, 0, root.Position)
	assert.Equal(t, "ordered", root.Routing, "a group of groups walked them one at a time")

	top := m20260827_childrenOf(rows, root.ID)
	require.Equal(t, []string{"Ferry Terminal", "The Wharf"}, m20260827_titles(top),
		"locations come before subgroups")
	assert.Equal(t, []int{0, 1}, []int{top[0].Position, top[1].Position})
	assert.Equal(t, "ferry-terminal", top[0].Slug, "a location keeps its slug")

	wharf := top[1]
	assert.Equal(t, "the-wharf", wharf.Slug)
	assert.Equal(t, "ordered", wharf.Routing, "it holds a subgroup, so it too walked them in order")
	require.NotNil(t, wharf.ChildrenMin)
	require.NotNil(t, wharf.ChildrenMax)
	assert.Equal(t, 1, *wharf.ChildrenMin)
	assert.Equal(t, 1, *wharf.ChildrenMax, "auto-advancing at the minimum is a closed band")

	inWharf := m20260827_childrenOf(rows, wharf.ID)
	require.Equal(t, []string{"Lighthouse", "Boat Shed", "Cargo Yard"}, m20260827_titles(inWharf))

	yard := inWharf[2]
	assert.Equal(t, "ordered", yard.Routing, "a group of locations keeps its own routing")
	assert.Nil(t, yard.ChildrenMin, "completion all leaves the band open")
	assert.Equal(t, []string{"Crane"}, m20260827_titles(m20260827_childrenOf(rows, yard.ID)))
}

func TestLocationsToObjectives_MovesBlocksAndSeedsTheScan(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Blocks")
	locID, code := m20260827_seedLocation(t, dbc, questID, "Fountain", "fountain", 15)
	quiz := m20260827_seedBlock(t, dbc, locID, "quiz", "location_content")
	text := m20260827_seedBlock(t, dbc, locID, "markdown", "location_content")
	hint := m20260827_seedBlock(t, dbc, locID, "markdown", "navigation")

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	objective := m20260827_childrenOf(rows, m20260827_root(t, rows).ID)[0]

	contextOf := func(id string) string {
		var c string
		require.NoError(t, dbc.QueryRowContext(ctx,
			`SELECT "context" FROM "blocks" WHERE "id" = ? AND "owner_id" = ?`, id, objective.ID).Scan(&c))
		return c
	}
	assert.Equal(t, "objective_proof", contextOf(quiz), "content that took input was the requirement")
	assert.Equal(t, "objective_reveal", contextOf(text), "the rest is shown once it is found")
	assert.Equal(t, "objective_proof", contextOf(hint), "a hint helps find it")

	var data string
	var points int
	var ordering int
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT "data", "points", "ordering" FROM "blocks" WHERE "owner_id" = ? AND "type" = 'scan'`,
		objective.ID).Scan(&data, &points, &ordering))
	var scan blocks.ScanBlock
	require.NoError(t, json.Unmarshal([]byte(data), &scan))
	assert.True(t, scan.Matches(code), "the code typed in")
	assert.True(t, scan.Matches("https://rapua.nz/s/"+code), "a v7 poster read by the camera")
	assert.Equal(t, 15, points, "the location's points move onto the scan")
	assert.Equal(t, -1, ordering)
}

func TestLocationsToObjectives_CarriesCheckInsOver(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Underway")
	locID, _ := m20260827_seedLocation(t, dbc, questID, "Museum", "museum", 20)
	finished := m20260827_seedRun(t, dbc, questID)
	midway := m20260827_seedRun(t, dbc, questID)
	untouched := m20260827_seedRun(t, dbc, questID)
	m20260827_seedCheckIn(t, dbc, finished, questID, locID, true)
	m20260827_seedCheckIn(t, dbc, midway, questID, locID, false)

	require.NoError(t, m20260827_up(ctx, dbc))

	scanState := func(runCode string) (bool, int, int) {
		var complete bool
		var awarded, n int
		n = m20260827_count(t, dbc,
			`SELECT count(*) FROM "run_block_states" s JOIN "blocks" b ON b."id" = s."block_id"`+
				` WHERE b."type" = 'scan' AND s."run_code" = ?`, runCode)
		if n == 0 {
			return false, 0, 0
		}
		require.NoError(t, dbc.QueryRowContext(ctx,
			`SELECT s."is_complete", s."points_awarded" FROM "run_block_states" s`+
				` JOIN "blocks" b ON b."id" = s."block_id" WHERE b."type" = 'scan' AND s."run_code" = ?`,
			runCode).Scan(&complete, &awarded))
		return complete, awarded, n
	}

	complete, awarded, n := scanState(finished)
	assert.Equal(t, 1, n)
	assert.True(t, complete, "a check-in is the find the scan asks for")
	assert.Equal(t, 20, awarded, "already paid into the run, so a rescan pays nothing")

	complete, _, n = scanState(midway)
	assert.Equal(t, 1, n)
	assert.True(t, complete)

	_, _, n = scanState(untouched)
	assert.Zero(t, n, "a team that never checked in has found nothing")

	completions := func(runCode string) []string {
		rows, err := dbc.QueryContext(ctx,
			`SELECT "context" FROM "objective_context_completions" WHERE "run_code" = ? ORDER BY "context"`, runCode)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		var out []string
		for rows.Next() {
			var c string
			require.NoError(t, rows.Scan(&c))
			out = append(out, c)
		}
		require.NoError(t, rows.Err())
		return out
	}
	assert.Equal(t, []string{"objective_proof", "objective_reveal"}, completions(finished),
		"completed blocks finished the location")
	assert.Empty(t, completions(midway), "its content blocks still decide the proof")
}

func TestLocationsToObjectives_SecretGroupsDissolveIntoTheirParent(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Trace")
	main, _ := m20260827_seedLocation(t, dbc, questID, "Library", "library", 10)
	bonus, _ := m20260827_seedLocation(t, dbc, questID, "Rooftop", "rooftop", 50)
	raw, err := json.Marshal(map[string]any{
		"is_root": true,
		"sub_groups": []any{
			map[string]any{"name": "Main", "routing": "ordered", "location_ids": []string{main}},
			map[string]any{"name": "Bonus locations", "routing": "secret", "location_ids": []string{bonus}},
		},
	})
	require.NoError(t, err)
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = ? WHERE "id" = ?`, string(raw), questID)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	top := m20260827_childrenOf(rows, m20260827_root(t, rows).ID)
	require.Equal(t, []string{"Main", "Rooftop"}, m20260827_titles(top),
		"the secret group leaves no section behind")
	assert.Equal(t, 1, top[1].Position)

	secretOf := func(id string) bool {
		var secret bool
		require.NoError(t, dbc.QueryRowContext(ctx,
			`SELECT "secret" FROM "objectives" WHERE "id" = ?`, id).Scan(&secret))
		return secret
	}
	assert.True(t, secretOf(top[1].ID))
	assert.False(t, secretOf(top[0].ID))
	assert.False(t, secretOf(m20260827_childrenOf(rows, top[0].ID)[0].ID))
}

// A group held no content, so a group inside a secret one is a child like any
// location: it becomes a secret section, a side quest once found, whose own
// contents play as usual.
func TestLocationsToObjectives_GroupsInASecretGroupBecomeSecretSections(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Trace")
	main, _ := m20260827_seedLocation(t, dbc, questID, "Library", "library", 10)
	bonus, _ := m20260827_seedLocation(t, dbc, questID, "Rooftop", "rooftop", 50)
	tunnel, tunnelCode := m20260827_seedLocation(t, dbc, questID, "Tunnel", "tunnel", 20)
	runCode := m20260827_seedRun(t, dbc, questID)
	m20260827_seedCheckIn(t, dbc, runCode, questID, tunnel, false)
	raw, err := json.Marshal(map[string]any{
		"is_root": true,
		"sub_groups": []any{
			map[string]any{"name": "Main", "routing": "ordered", "location_ids": []string{main}},
			map[string]any{
				"name": "Bonus", "routing": "secret", "location_ids": []string{bonus},
				"sub_groups": []any{
					map[string]any{"name": "Undercroft", "routing": "free_roam", "location_ids": []string{tunnel}},
				},
			},
		},
	})
	require.NoError(t, err)
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = ? WHERE "id" = ?`, string(raw), questID)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	top := m20260827_childrenOf(rows, m20260827_root(t, rows).ID)
	require.Equal(t, []string{"Main", "Rooftop", "Undercroft"}, m20260827_titles(top))

	secretOf := func(id string) bool {
		var secret bool
		require.NoError(t, dbc.QueryRowContext(ctx,
			`SELECT "secret" FROM "objectives" WHERE "id" = ?`, id).Scan(&secret))
		return secret
	}
	assert.True(t, secretOf(top[2].ID), "the nested group is a secret in the dissolved group's place")
	inside := m20260827_childrenOf(rows, top[2].ID)
	require.Equal(t, []string{"Tunnel"}, m20260827_titles(inside))
	assert.False(t, secretOf(inside[0].ID), "its own contents play as usual once found")

	// The section is found by its first poster, as the way in was in v7.
	var scanID, data string
	var points int
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT "id", "data", "points" FROM "blocks"`+
			` WHERE "owner_id" = ? AND "type" = 'scan' AND "context" = 'objective_proof'`,
		top[2].ID).Scan(&scanID, &data, &points))
	var scan blocks.ScanBlock
	require.NoError(t, json.Unmarshal([]byte(data), &scan))
	assert.True(t, scan.Matches(tunnelCode))
	assert.True(t, scan.Matches("https://rapua.nz/s/"+tunnelCode), "an old poster link still opens it")
	assert.Zero(t, points, "the location keeps the points")

	var found bool
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT "is_complete" FROM "run_block_states" WHERE "run_code" = ? AND "block_id" = ?`,
		runCode, scanID).Scan(&found))
	assert.True(t, found, "a team that checked in there has already found it")
}

// The editor posts a visible row's index in its own list, which is only its
// position while secrets keep to the tail.
func TestLocationsToObjectives_SecretsSettleAtTheTail(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Trace")
	main, _ := m20260827_seedLocation(t, dbc, questID, "Library", "library", 10)
	bonus, _ := m20260827_seedLocation(t, dbc, questID, "Rooftop", "rooftop", 50)
	m20260827_seedLocation(t, dbc, questID, "Stray", "stray", 0)
	raw, err := json.Marshal(map[string]any{
		"is_root": true, "completion_type": "minimum", "minimum_required": 3, "auto_advance": true,
		"sub_groups": []any{
			map[string]any{"name": "Bonus locations", "routing": "secret", "location_ids": []string{bonus}},
			map[string]any{"name": "Main", "routing": "ordered", "location_ids": []string{main}},
		},
	})
	require.NoError(t, err)
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = ? WHERE "id" = ?`, string(raw), questID)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	root := m20260827_root(t, rows)
	top := m20260827_childrenOf(rows, root.ID)
	require.Equal(t, []string{"Main", "Stray", "Rooftop"}, m20260827_titles(top),
		"listed or not, visible rows come before the secret")
	for i, row := range top {
		assert.Equal(t, i, row.Position)
	}
	require.NotNil(t, root.ChildrenMin)
	require.NotNil(t, root.ChildrenMax)
	assert.Equal(t, 2, *root.ChildrenMin, "the band asks no more than the visible rows")
	assert.Equal(t, 2, *root.ChildrenMax)
}

func TestLocationsToObjectives_CheckpointBlocksBecomeProof(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Checkpoints")
	locID, _ := m20260827_seedLocation(t, dbc, questID, "Gate", "gate", 0)
	password := m20260827_seedBlock(t, dbc, locID, "password", "checkpoint")

	require.NoError(t, m20260827_up(ctx, dbc))

	var blockContext string
	require.NoError(t, dbc.QueryRowContext(ctx,
		`SELECT "context" FROM "blocks" WHERE "id" = ?`, password).Scan(&blockContext))
	assert.Equal(t, "objective_proof", blockContext)
}

func TestLocationsToObjectives_RoutingFor(t *testing.T) {
	for _, tc := range []struct {
		name  string
		group m20260827_group
		want  string
	}{
		{"empty was the engine default", m20260827_group{}, "free_roam"},
		{"unknown values play as the default did", m20260827_group{Routing: "custom"}, "free_roam"},
		{"ordered is kept", m20260827_group{Routing: "ordered"}, "ordered"},
		{"randomised is kept", m20260827_group{Routing: "randomised"}, "randomised"},
		{"a group of groups is ordered", m20260827_group{
			Routing: "free_roam", SubGroups: []m20260827_group{{}},
		}, "ordered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, m20260827_routingFor(tc.group))
		})
	}
}

func TestLocationsToObjectives_EveryQuestGetsARoot(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	empty := m20260827_seedQuest(t, dbc, "No Blob")
	unreadable := m20260827_seedQuest(t, dbc, "Bad Blob")
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = '{not json' WHERE "id" = ?`, unreadable)
	m20260827_seedLocation(t, dbc, unreadable, "Gate", "gate", 0)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, empty)
	assert.Empty(t, m20260827_childrenOf(rows, m20260827_root(t, rows).ID))

	rows = m20260827_rows(t, dbc, unreadable)
	children := m20260827_childrenOf(rows, m20260827_root(t, rows).ID)
	require.Len(t, children, 1, "a location the blob cannot place goes under the root")
	assert.Equal(t, "gate", children[0].Slug)
}

func TestLocationsToObjectives_AdoptsUnmentionedAndSkipsStale(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Gaps")
	listed, _ := m20260827_seedLocation(t, dbc, questID, "Listed", "listed", 0)
	m20260827_seedLocation(t, dbc, questID, "Forgotten", "forgotten", 0)
	raw, err := json.Marshal(map[string]any{
		"is_root":      true,
		"location_ids": []string{gofakeit.UUID(), listed, listed},
	})
	require.NoError(t, err)
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = ? WHERE "id" = ?`, string(raw), questID)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	children := m20260827_childrenOf(rows, m20260827_root(t, rows).ID)
	require.Equal(t, []string{"Listed", "Forgotten"}, m20260827_titles(children),
		"a stale id and a repeat place nothing, and the unmentioned location goes last")
	assert.Equal(t, []int{0, 1}, []int{children[0].Position, children[1].Position}, "positions stay dense")
}

func TestLocationsToObjectives_SectionsGiveWayOnSlugs(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Slugs")
	clash, _ := m20260827_seedLocation(t, dbc, questID, "Garden", "garden", 0)
	blank, _ := m20260827_seedLocation(t, dbc, questID, "Rose Bed", "", 0)
	raw, err := json.Marshal(map[string]any{
		"is_root": true,
		"sub_groups": []any{
			map[string]any{"name": "Garden", "location_ids": []string{clash}},
			map[string]any{"name": "Garden", "location_ids": []string{blank}},
		},
	})
	require.NoError(t, err)
	m20260827_exec(t, dbc, `UPDATE "quests" SET "game_structure" = ? WHERE "id" = ?`, string(raw), questID)

	require.NoError(t, m20260827_up(ctx, dbc))

	rows := m20260827_rows(t, dbc, questID)
	sections := m20260827_childrenOf(rows, m20260827_root(t, rows).ID)
	require.Len(t, sections, 2)
	assert.Equal(t, "garden-2", sections[0].Slug, "the location already holds garden")
	assert.Equal(t, "garden-3", sections[1].Slug)
	assert.Equal(t, "garden", m20260827_childrenOf(rows, sections[0].ID)[0].Slug)
	assert.Equal(t, "rose-bed", m20260827_childrenOf(rows, sections[1].ID)[0].Slug,
		"a location with no slug gets one, or it has no address")
}

func TestLocationsToObjectives_MissingMarkerFailsThatQuestOnly(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	healthy := m20260827_seedQuest(t, dbc, "Healthy")
	m20260827_seedLocation(t, dbc, healthy, "Fine", "fine", 0)

	broken := m20260827_seedQuest(t, dbc, "Broken")
	locID, code := m20260827_seedLocation(t, dbc, broken, "Lost", "lost", 0)
	conn, err := dbc.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF")
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `DELETE FROM "markers" WHERE "code" = ?`, code)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	err = m20260827_up(ctx, dbc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), locID)

	assert.NotEmpty(t, m20260827_rows(t, dbc, healthy), "the other quest was committed")
	assert.Empty(t, m20260827_rows(t, dbc, broken), "the failed quest was rolled back whole")

	err = m20260829010000_up(ctx, dbc)
	require.Error(t, err, "dropping locations now would destroy the unconverted one")
	assert.True(t, tableExists(ctx, dbc, "locations"))
}

func TestLocationsToObjectives_RerunIsANoOp(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Twice")
	locID, _ := m20260827_seedLocation(t, dbc, questID, "Once", "once", 0)
	m20260827_seedCheckIn(t, dbc, m20260827_seedRun(t, dbc, questID), questID, locID, true)

	require.NoError(t, m20260827_up(ctx, dbc))
	before := m20260827_rows(t, dbc, questID)
	require.NoError(t, m20260827_up(ctx, dbc))

	assert.Equal(t, before, m20260827_rows(t, dbc, questID))
	assert.Equal(t, 1, m20260827_count(t, dbc, `SELECT count(*) FROM "blocks" WHERE "type" = 'scan'`))
}

func TestDropLocationMigration_DropsTablesOnceConverted(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	questID := m20260827_seedQuest(t, dbc, "Drop")
	m20260827_seedLocation(t, dbc, questID, "Gone", "gone", 0)
	require.NoError(t, m20260827_up(ctx, dbc))
	before := m20260827_rows(t, dbc, questID)

	require.NoError(t, m20260829010000_up(ctx, dbc))

	assert.False(t, tableExists(ctx, dbc, "locations"))
	assert.False(t, tableExists(ctx, dbc, "markers"))
	assert.False(t, tableExists(ctx, dbc, "check_ins"))
	assert.False(t, columnExists(ctx, dbc, "runs", "must_scan_out"))
	assert.False(t, columnExists(ctx, dbc, "uploads", "location_id"))
	assert.False(t, columnExists(ctx, dbc, "quest_settings", "must_check_out"))
	assert.Equal(t, before, m20260827_rows(t, dbc, questID), "the converted content survives the drop")

	require.NoError(t, m20260829010000_up(ctx, dbc), "rerunning once the tables are gone is safe")
}

func TestDropLocationMigration_DownRestoresSchema(t *testing.T) {
	dbc := m20260827_setupPreConversion(t)
	ctx := context.Background()

	require.NoError(t, m20260827_up(ctx, dbc))
	require.NoError(t, m20260829010000_up(ctx, dbc))
	require.NoError(t, m20260829010000_down(ctx, dbc))

	assert.True(t, tableExists(ctx, dbc, "locations"))
	assert.True(t, tableExists(ctx, dbc, "markers"))
	assert.True(t, tableExists(ctx, dbc, "check_ins"))
	assert.True(t, columnExists(ctx, dbc, "runs", "must_scan_out"))
	assert.True(t, columnExists(ctx, dbc, "uploads", "location_id"))
	assert.True(t, columnExists(ctx, dbc, "quest_settings", "must_check_out"))
}
