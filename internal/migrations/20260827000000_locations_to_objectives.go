package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260827_up, m20260827_down)
}

// m20260827_interactiveBlockTypes is frozen: one of these in a location's
// content was the real requirement, so it moves into proof.
var m20260827_interactiveBlockTypes = map[string]bool{
	"broker": true, "checklist": true, "choice": true, "clue": true,
	"free_text": true, "password": true, "photo": true, "pincode": true,
	"quiz": true, "rating": true, "scan": true, "sorting": true,
}

// m20260827_group is a snapshot, since nothing else knows the blob any more.
type m20260827_group struct {
	Name            string            `json:"name"`
	Routing         string            `json:"routing"`
	MaxNext         int               `json:"max_next"`
	CompletionType  string            `json:"completion_type"`
	MinimumRequired int               `json:"minimum_required"`
	AutoAdvance     bool              `json:"auto_advance"`
	LocationIDs     []string          `json:"location_ids"`
	SubGroups       []m20260827_group `json:"sub_groups"`
}

type m20260827_quest struct {
	ID            string
	Name          string
	GameStructure string
}

type m20260827_location struct {
	ID       string
	Name     string
	Slug     string
	MarkerID string
	Points   int
}

type m20260827_block struct {
	ID      string
	Type    string
	Context string
}

type m20260827_checkIn struct {
	RunCode string
	// Opaque: the driver returns either a string or a time for it.
	CompletedAt     any
	BlocksCompleted bool
}

var m20260827_nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// m20260827_up carries check-ins over because runs are underway at deploy:
// without them teams lose their progress and a rescan pays the points twice.
//
// A quest is one transaction, skipped once it holds an objective, so a rerun
// picks up only the quests that failed.
func m20260827_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "locations") || !columnExists(ctx, db, "quests", "game_structure") {
		return nil
	}

	quests, err := m20260827_loadQuests(ctx, db)
	if err != nil {
		return err
	}

	var failed []string
	for _, q := range quests {
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			return m20260827_convertQuest(ctx, tx, q)
		})
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", q.ID, err))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf(
			"failed to convert %d quest(s), the rest were committed and a rerun skips them: %v",
			len(failed), failed,
		)
	}
	return nil
}

func m20260827_loadQuests(ctx context.Context, db *bun.DB) ([]m20260827_quest, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT "id", COALESCE("name", ''), COALESCE("game_structure", '') FROM "quests"`)
	if err != nil {
		return nil, fmt.Errorf("loading quests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var quests []m20260827_quest
	for rows.Next() {
		var q m20260827_quest
		if err := rows.Scan(&q.ID, &q.Name, &q.GameStructure); err != nil {
			return nil, fmt.Errorf("scanning quest: %w", err)
		}
		quests = append(quests, q)
	}
	return quests, rows.Err()
}

type m20260827_converter struct {
	tx         bun.Tx
	quest      m20260827_quest
	locations  map[string]m20260827_location
	order      []string
	placed     map[string]bool
	takenSlugs map[string]bool
}

func m20260827_convertQuest(ctx context.Context, tx bun.Tx, q m20260827_quest) error {
	var existing int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM "objectives" WHERE "quest_id" = ?`, q.ID).Scan(&existing); err != nil {
		return fmt.Errorf("counting objectives: %w", err)
	}
	if existing > 0 {
		return nil
	}

	c := m20260827_converter{
		tx:         tx,
		quest:      q,
		locations:  map[string]m20260827_location{},
		placed:     map[string]bool{},
		takenSlugs: map[string]bool{},
	}
	if err := c.loadLocations(ctx); err != nil {
		return err
	}

	// An unreadable blob still gets a root: without one the quest cannot be
	// edited or played.
	var root m20260827_group
	if q.GameStructure != "" {
		if err := json.Unmarshal([]byte(q.GameStructure), &root); err != nil {
			root = m20260827_group{}
		}
	}

	rootID, next, err := c.convertGroup(ctx, root, nil, 0)
	if err != nil {
		return err
	}
	// Unlisted, a location would be a second root, so it goes under this one.
	for _, locID := range c.order {
		if c.placed[locID] {
			continue
		}
		if err := c.convertLocation(ctx, c.locations[locID], rootID, next); err != nil {
			return err
		}
		next++
	}
	return nil
}

// loadLocations reserves location slugs first: they are in links and print, so
// a section gives way on a collision.
func (c *m20260827_converter) loadLocations(ctx context.Context) error {
	rows, err := c.tx.QueryContext(ctx,
		`SELECT "id", COALESCE("name", ''), COALESCE("slug", ''), COALESCE("marker_id", ''), COALESCE("points", 0)`+
			` FROM "locations" WHERE "quest_id" = ? ORDER BY "order", "id"`, c.quest.ID)
	if err != nil {
		return fmt.Errorf("loading locations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var loc m20260827_location
		if err := rows.Scan(&loc.ID, &loc.Name, &loc.Slug, &loc.MarkerID, &loc.Points); err != nil {
			return fmt.Errorf("scanning location: %w", err)
		}
		c.locations[loc.ID] = loc
		c.order = append(c.order, loc.ID)
		if loc.Slug != "" {
			c.takenSlugs[loc.Slug] = true
		}
	}
	return rows.Err()
}

func (c *m20260827_converter) convertGroup(
	ctx context.Context, group m20260827_group, parentID *string, position int,
) (string, int, error) {
	title := group.Name
	if parentID == nil && title == "" {
		title = c.quest.Name
	}
	if title == "" {
		title = "Untitled section"
	}
	slugSeed := m20260827_slugify(title)
	if parentID == nil && slugSeed == "" {
		slugSeed = "root"
	}

	minChildren, maxChildren := m20260827_band(group)
	sectionID := uuid.New().String()
	if _, err := c.tx.ExecContext(ctx,
		`INSERT INTO "objectives" ("id", "quest_id", "slug", "title", "parent_id", "position",`+
			` "routing", "max_next", "children_min", "children_max")`+
			` VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sectionID, c.quest.ID, m20260827_uniqueSlug(slugSeed, "section", c.takenSlugs), title,
		parentID, position, m20260827_routingFor(group), group.MaxNext, minChildren, maxChildren,
	); err != nil {
		return "", 0, fmt.Errorf("inserting section %q: %w", title, err)
	}

	child := 0
	for _, locID := range group.LocationIDs {
		loc, ok := c.locations[locID]
		if !ok || c.placed[locID] {
			continue
		}
		if err := c.convertLocation(ctx, loc, sectionID, child); err != nil {
			return "", 0, err
		}
		child++
	}
	for _, sub := range group.SubGroups {
		if _, _, err := c.convertGroup(ctx, sub, &sectionID, child); err != nil {
			return "", 0, err
		}
		child++
	}
	return sectionID, child, nil
}

// convertLocation puts the points on the scan block because an objective has
// none of its own.
func (c *m20260827_converter) convertLocation(
	ctx context.Context, loc m20260827_location, parentID string, position int,
) error {
	c.placed[loc.ID] = true

	var markerExists bool
	if err := c.tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM "markers" WHERE "code" = ?)`, loc.MarkerID).Scan(&markerExists); err != nil {
		return fmt.Errorf("checking marker for location %s: %w", loc.ID, err)
	}
	if !markerExists {
		// Fail rather than guess: 20260829010000 drops locations.
		return fmt.Errorf("location %s has no marker to seed its scan block with", loc.ID)
	}

	slug := loc.Slug
	if slug == "" {
		slug = m20260827_uniqueSlug(m20260827_slugify(loc.Name), "objective", c.takenSlugs)
	}

	objID := uuid.New().String()
	if _, err := c.tx.ExecContext(ctx,
		`INSERT INTO "objectives" ("id", "quest_id", "slug", "title", "parent_id", "position", "routing")`+
			` VALUES (?, ?, ?, ?, ?, ?, 'ordered')`,
		objID, c.quest.ID, slug, loc.Name, parentID, position,
	); err != nil {
		return fmt.Errorf("inserting objective for location %s: %w", loc.ID, err)
	}

	if err := c.moveBlocks(ctx, loc.ID, objID); err != nil {
		return err
	}

	scanID, err := c.insertScanBlock(ctx, loc, objID)
	if err != nil {
		return err
	}
	return c.carryCheckIns(ctx, loc, objID, scanID)
}

// loadBlocks reads in full first: the transaction has one connection, and an
// open cursor blocks the updates.
func (c *m20260827_converter) loadBlocks(ctx context.Context, locID string) ([]m20260827_block, error) {
	rows, err := c.tx.QueryContext(ctx,
		`SELECT "id", "type", "context" FROM "blocks" WHERE "owner_id" = ?`, locID)
	if err != nil {
		return nil, fmt.Errorf("loading blocks for location %s: %w", locID, err)
	}
	defer func() { _ = rows.Close() }()

	var blks []m20260827_block
	for rows.Next() {
		var b m20260827_block
		if err := rows.Scan(&b.ID, &b.Type, &b.Context); err != nil {
			return nil, fmt.Errorf("scanning block: %w", err)
		}
		blks = append(blks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading blocks for location %s: %w", locID, err)
	}
	return blks, nil
}

func (c *m20260827_converter) moveBlocks(ctx context.Context, locID, objID string) error {
	blks, err := c.loadBlocks(ctx, locID)
	if err != nil {
		return err
	}

	for _, b := range blks {
		newContext := "objective_reveal"
		if b.Context == "navigation" || (b.Context == "location_content" && m20260827_interactiveBlockTypes[b.Type]) {
			newContext = "objective_proof"
		}
		if _, err := c.tx.ExecContext(ctx,
			`UPDATE "blocks" SET "owner_id" = ?, "context" = ? WHERE "id" = ?`, objID, newContext, b.ID,
		); err != nil {
			return fmt.Errorf("re-owning block %s: %w", b.ID, err)
		}
	}
	return nil
}

func (c *m20260827_converter) insertScanBlock(
	ctx context.Context, loc m20260827_location, objID string,
) (string, error) {
	data, err := json.Marshal(map[string]any{
		"prompt": "Scan the code to prove you found it",
		"codes":  []map[string]any{{"value": loc.MarkerID, "generate": false}},
		"match":  "exact",
	})
	if err != nil {
		return "", fmt.Errorf("encoding scan block for location %s: %w", loc.ID, err)
	}
	scanID := uuid.New().String()
	// -1 sorts it ahead of the moved blocks.
	if _, err := c.tx.ExecContext(ctx,
		`INSERT INTO "blocks"`+
			` ("id", "owner_id", "type", "context", "data", "ordering", "points", "validation_required")`+
			` VALUES (?, ?, 'scan', 'objective_proof', ?, -1, ?, TRUE)`,
		scanID, objID, string(data), loc.Points,
	); err != nil {
		return "", fmt.Errorf("inserting scan block for location %s: %w", loc.ID, err)
	}
	return scanID, nil
}

// carryCheckIns records the points as awarded, since the run already holds them.
// An unfinished check-in leaves proof to the content blocks, whose states moved
// with them.
func (c *m20260827_converter) carryCheckIns(
	ctx context.Context, loc m20260827_location, objID, scanID string,
) error {
	checkIns, err := c.loadCheckIns(ctx, loc.ID)
	if err != nil {
		return err
	}

	scanState, err := json.Marshal(map[string]any{"attempts": 1, "scans": []string{loc.MarkerID}})
	if err != nil {
		return fmt.Errorf("encoding scan state for location %s: %w", loc.ID, err)
	}

	for _, ci := range checkIns {
		if _, err := c.tx.ExecContext(ctx,
			`INSERT INTO "run_block_states"`+
				` ("run_code", "block_id", "quest_id", "is_complete", "points_awarded", "player_data", "created_at", "updated_at")`+
				` VALUES (?, ?, ?, TRUE, ?, ?, ?, ?)`,
			ci.RunCode, scanID, c.quest.ID, loc.Points, string(scanState), ci.CompletedAt, ci.CompletedAt,
		); err != nil {
			return fmt.Errorf("recording scan for run %s at location %s: %w", ci.RunCode, loc.ID, err)
		}
		if !ci.BlocksCompleted {
			continue
		}
		for _, blockContext := range []string{"objective_proof", "objective_reveal"} {
			if _, err := c.tx.ExecContext(ctx,
				`INSERT INTO "objective_context_completions" ("run_code", "objective_id", "context", "completed_at")`+
					` VALUES (?, ?, ?, ?)`,
				ci.RunCode, objID, blockContext, ci.CompletedAt,
			); err != nil {
				return fmt.Errorf("recording %s for run %s at location %s: %w", blockContext, ci.RunCode, loc.ID, err)
			}
		}
	}
	return nil
}

func (c *m20260827_converter) loadCheckIns(ctx context.Context, locID string) ([]m20260827_checkIn, error) {
	rows, err := c.tx.QueryContext(ctx,
		`SELECT "run_code", COALESCE("time_in", "created_at"), COALESCE("blocks_completed", 0)`+
			` FROM "check_ins" WHERE "location_id" = ?`, locID)
	if err != nil {
		return nil, fmt.Errorf("loading check-ins for location %s: %w", locID, err)
	}
	defer func() { _ = rows.Close() }()

	var checkIns []m20260827_checkIn
	for rows.Next() {
		var ci m20260827_checkIn
		if err := rows.Scan(&ci.RunCode, &ci.CompletedAt, &ci.BlocksCompleted); err != nil {
			return nil, fmt.Errorf("scanning check-in: %w", err)
		}
		checkIns = append(checkIns, ci)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading check-ins for location %s: %w", locID, err)
	}
	return checkIns, nil
}

// m20260827_routingFor makes a group of groups ordered: v7 walked subgroups one
// at a time whatever its routing said. Empty and secret both played as free
// roam.
func m20260827_routingFor(group m20260827_group) string {
	if len(group.SubGroups) > 0 {
		return "ordered"
	}
	switch group.Routing {
	case "ordered", "randomised", "free_roam":
		return group.Routing
	default:
		return "free_roam"
	}
}

// m20260827_band:
//
//	completion=all                        -> both bounds omitted, every child
//	completion=minimum k, auto_advance    -> [k, k], auto-completing at k
//	completion=minimum k, no auto_advance -> [k, omitted], the player finishes
func m20260827_band(group m20260827_group) (*int, *int) {
	if group.CompletionType != "minimum" {
		return nil, nil
	}
	minChildren := group.MinimumRequired
	if group.AutoAdvance {
		maxChildren := minChildren
		return &minChildren, &maxChildren
	}
	return &minChildren, nil
}

func m20260827_slugify(name string) string {
	return strings.Trim(m20260827_nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func m20260827_uniqueSlug(candidate, fallback string, taken map[string]bool) string {
	if candidate == "" {
		candidate = fallback
	}
	slug := candidate
	for n := 2; taken[slug]; n++ {
		slug = fmt.Sprintf("%s-%d", candidate, n)
	}
	taken[slug] = true
	return slug
}

// m20260827_down is a no-op: 20260829010000's down restores empty tables, so
// there is nothing to move the blocks back onto.
func m20260827_down(_ context.Context, _ *bun.DB) error {
	return nil
}
