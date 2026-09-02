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
	Migrations.MustRegister(m20260902000000_up, m20260902000000_down)
}

// m20260902000000_group is the shape stored in quests.game_structure, read here
// as plain JSON rather than through models.GameStructure so this migration
// keeps working once that type changes or goes.
type m20260902000000_group struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Color       string          `json:"color"`
	Routing     string          `json:"routing"`
	MaxNext     int             `json:"max_next"`
	FinishLabel string          `json:"finish_label"`
	Depends     json.RawMessage `json:"depends"`

	// The completion band as written, when the blob is new enough to carry it.
	ChildrenMin *int `json:"children_min"`
	ChildrenMax *int `json:"children_max"`

	// The older spelling of the same thing, which cannot express a range.
	CompletionType  string `json:"completion_type"`
	MinimumRequired int    `json:"minimum_required"`
	AutoAdvance     bool   `json:"auto_advance"`

	ObjectiveIDs []string                `json:"objective_ids"`
	SubGroups    []m20260902000000_group `json:"sub_groups"`
}

type m20260902000000_quest struct {
	bun.BaseModel `bun:"table:quests"`

	ID            string `bun:"id,pk"`
	Name          string `bun:"name"`
	GameStructure string `bun:"game_structure"`
}

type m20260902000000_objective struct {
	bun.BaseModel `bun:"table:objectives"`

	ID       string `bun:"id,pk"`
	QuestID  string `bun:"quest_id"`
	ParentID string `bun:"parent_id"`
	Position int    `bun:"position"`
	Slug     string `bun:"slug"`
	Title    string `bun:"title"`
}

var m20260902000000_nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// m20260902000000_up moves each quest's structure out of the group blob and
// into the objectives table, where the tree is parent_id and position.
//
// Every group becomes an objective row: a section is an objective with
// children, so the blob's separate arrays collapse into one ordered sibling
// list. Objectives come before subgroups within a node, matching the order the
// blob itself documents.
//
// Two things in the stored blobs are dropped rather than converted.
// location_ids point at rows deleted with the Location model and resolve to
// nothing. A group's `when` was never authorable, and the grammar it belonged
// to is gone.
func m20260902000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "quests", "game_structure") {
		return nil
	}

	var quests []m20260902000000_quest
	err := db.NewSelect().
		Model(&quests).
		Column("id", "name", "game_structure").
		Where("game_structure IS NOT NULL AND game_structure != ''").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("loading quests: %w", err)
	}

	for _, quest := range quests {
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			return m20260902000000_convertQuest(ctx, tx, quest)
		})
		if err != nil {
			return fmt.Errorf("quest %s: %w", quest.ID, err)
		}
	}
	return nil
}

// m20260902000000_convertQuest converts one quest, and is always called inside
// a transaction: the guard below reads "already placed" as "already converted",
// which is only true if a quest can never be left half done.
func m20260902000000_convertQuest(ctx context.Context, db bun.IDB, quest m20260902000000_quest) error {
	var root m20260902000000_group
	if err := json.Unmarshal([]byte(quest.GameStructure), &root); err != nil {
		// A blob that cannot be read was not driving the game either.
		return nil //nolint:nilerr // unreadable stored JSON is skipped, not fatal
	}
	var existing []m20260902000000_objective
	err := db.NewSelect().
		Model(&existing).
		Column("id", "slug", "parent_id").
		Where("quest_id = ?", quest.ID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("loading objectives: %w", err)
	}

	knownObjectives := make(map[string]bool, len(existing))
	takenSlugs := make(map[string]bool, len(existing))
	for _, obj := range existing {
		// Anything already placed means this quest has been converted, which a
		// rerun after a partial failure has to notice: the blob's ids cannot
		// serve as a marker because they are not unique.
		if obj.ParentID != "" {
			return nil
		}
		knownObjectives[obj.ID] = true
		if obj.Slug != "" {
			takenSlugs[obj.Slug] = true
		}
	}

	placed := make(map[string]bool, len(existing))
	rootID, err := m20260902000000_convertGroupReturningID(
		ctx, db, quest, root, "", 0, knownObjectives, takenSlugs, placed,
	)
	if err != nil {
		return err
	}

	// An objective the blob never mentioned would be left with no parent, which
	// reads as a second root. The tree it belongs in is unknowable, so it goes
	// under the root where a player can still reach it.
	//
	// Counting what was actually placed rather than what the blob declared
	// keeps positions dense: a stale id in the root's list places nothing, and
	// starting past it would leave a hole where it would have sat.
	position, err := db.NewSelect().
		Model((*m20260902000000_objective)(nil)).
		Where("parent_id = ?", rootID).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("counting root children: %w", err)
	}
	for _, obj := range existing {
		if placed[obj.ID] {
			continue
		}
		if err := m20260902000000_place(ctx, db, obj.ID, rootID, position); err != nil {
			return err
		}
		position++
	}
	return nil
}

// m20260902000000_convertGroupReturningID writes one group as an objective row,
// places everything below it, and returns the new row's id so its children can
// name it as their parent.
func m20260902000000_convertGroupReturningID(
	ctx context.Context,
	db bun.IDB,
	quest m20260902000000_quest,
	group m20260902000000_group,
	parentID string,
	position int,
	knownObjectives, takenSlugs, placed map[string]bool,
) (string, error) {
	title := group.Name
	if title == "" {
		// Only the root has no name of its own, and the quest's name is the
		// most useful thing to call it.
		title = quest.Name
	}
	if title == "" {
		title = "Untitled section"
	}

	// A blob new enough to carry a slug keeps it, so a depends list naming a
	// section by slug still resolves after the move.
	slugSeed := group.Slug
	if slugSeed == "" {
		slugSeed = m20260902000000_slugify(title)
	}
	if parentID == "" && slugSeed == "" {
		slugSeed = "root"
	}

	// A fresh id rather than the blob's own: duplicating a quest copied the
	// blob verbatim, so the same group id appears in several quests, and an
	// objective id is unique across all of them.
	sectionID := uuid.New().String()
	row := &m20260902000000_objective{
		ID:       sectionID,
		QuestID:  quest.ID,
		ParentID: parentID,
		Position: position,
		Slug:     m20260902000000_uniqueSlug(slugSeed, takenSlugs),
		Title:    title,
	}
	if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
		return "", fmt.Errorf("inserting section %q: %w", title, err)
	}
	minChildren, maxChildren := m20260902000000_band(group)
	if err := m20260902000000_setSectionFields(ctx, db, sectionID, group, minChildren, maxChildren); err != nil {
		return "", err
	}

	// Objectives first, then subgroups: the order the blob's two arrays imply.
	childPosition := 0
	for _, objectiveID := range group.ObjectiveIDs {
		// An id left over from a deleted objective places nothing.
		if !knownObjectives[objectiveID] || placed[objectiveID] {
			continue
		}
		if err := m20260902000000_place(ctx, db, objectiveID, sectionID, childPosition); err != nil {
			return "", err
		}
		placed[objectiveID] = true
		childPosition++
	}

	for _, sub := range group.SubGroups {
		if _, err := m20260902000000_convertGroupReturningID(
			ctx, db, quest, sub, sectionID, childPosition, knownObjectives, takenSlugs, placed,
		); err != nil {
			return "", err
		}
		childPosition++
	}
	return sectionID, nil
}

// m20260902000000_routingFor returns the routing to store for a converted
// group.
//
// A group's stored routing only ever ordered its own objectives: the engine
// walked subgroups one at a time regardless of what the value said, so a root
// holding chapters read "free_roam" while presenting exactly one. Routing now
// governs every child alike, so copying that value verbatim would open every
// chapter at once on the first request: faithful to the record, and a different
// game.
//
// A group that held subgroups therefore migrates as ordered, which is what its
// players actually saw. One that held only objectives keeps its own value,
// which is the only thing it ever meant.
func m20260902000000_routingFor(group m20260902000000_group) string {
	if len(group.SubGroups) > 0 {
		return "ordered"
	}
	return group.Routing
}

// m20260902000000_setSectionFields writes the columns the snapshot model does
// not carry, so the insert above stays a plain row write.
func m20260902000000_setSectionFields(
	ctx context.Context, db bun.IDB, sectionID string, group m20260902000000_group, minChildren, maxChildren *int,
) error {
	var depends any
	if len(group.Depends) > 0 && string(group.Depends) != "null" {
		depends = string(group.Depends)
	}
	_, err := db.ExecContext(ctx,
		`UPDATE "objectives" SET "routing" = ?, "color" = ?, "max_next" = ?,`+
			` "children_min" = ?, "children_max" = ?, "finish_label" = ?, "depends" = ?`+
			` WHERE "id" = ?`,
		m20260902000000_routingFor(group), group.Color, group.MaxNext, minChildren, maxChildren,
		group.FinishLabel, depends, sectionID)
	if err != nil {
		return fmt.Errorf("setting section fields on %s: %w", sectionID, err)
	}
	return nil
}

func m20260902000000_place(ctx context.Context, db bun.IDB, objectiveID, parentID string, position int) error {
	_, err := db.ExecContext(ctx,
		`UPDATE "objectives" SET "parent_id" = ?, "position" = ? WHERE "id" = ?`,
		parentID, position, objectiveID)
	if err != nil {
		return fmt.Errorf("placing objective %s: %w", objectiveID, err)
	}
	return nil
}

// m20260902000000_band reads a group's completion band.
//
// A blob written since the band existed carries it verbatim, and that is taken
// as written: only it can express a range. An older blob carries just the
// completion trio, which is read as:
//
//	completion=all                        -> both bounds omitted, every child
//	completion=minimum k, auto_advance    -> [k, k], auto-completing at k
//	completion=minimum k, no auto_advance -> [k, omitted], the player finishes
func m20260902000000_band(group m20260902000000_group) (*int, *int) {
	if group.ChildrenMin != nil || group.ChildrenMax != nil {
		return group.ChildrenMin, group.ChildrenMax
	}
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

func m20260902000000_slugify(name string) string {
	return strings.Trim(m20260902000000_nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// m20260902000000_uniqueSlug keeps within the unique index on
// (quest_id, slug): group names repeat both across and within a quest.
func m20260902000000_uniqueSlug(candidate string, taken map[string]bool) string {
	if candidate == "" {
		candidate = "section"
	}
	slug := candidate
	for n := 2; taken[slug]; n++ {
		slug = fmt.Sprintf("%s-%d", candidate, n)
	}
	taken[slug] = true
	return slug
}

// m20260902000000_down removes the rows the up created and unplaces the rest.
// The blob is left untouched by both directions, so it remains the record this
// was derived from.
//
// Sections are identified as the rows something names as its parent, which
// holds because this migration is what first writes parent_id. An objective
// given a parent by anything else would be deleted here as though it were a
// section, so this direction is only sound while that remains true.
func m20260902000000_down(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "parent_id") {
		return nil
	}

	// A section is an objective row with children and no blocks of its own,
	// which is exactly what this migration inserted.
	if _, err := db.ExecContext(ctx, `DELETE FROM "objectives" WHERE "id" IN (
		SELECT "parent_id" FROM "objectives" WHERE "parent_id" IS NOT NULL AND "parent_id" != ''
	)`); err != nil {
		return fmt.Errorf("deleting section rows: %w", err)
	}
	if _, err := db.ExecContext(
		ctx, `UPDATE "objectives" SET "parent_id" = NULL, "position" = 0`,
	); err != nil {
		return fmt.Errorf("unplacing objectives: %w", err)
	}
	return nil
}
