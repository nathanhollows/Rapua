package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// m20260827_interactiveBlockTypes freezes, as of this migration's authorship,
// which block types require player input. Used only to decide whether a
// migrated Location.Content block becomes a proof requirement or reveal
// flavour. Block types added after this migration ships are irrelevant to
// it, since it only ever processes blocks that already existed when it runs.
var m20260827_interactiveBlockTypes = map[string]bool{
	"broker": true, "checklist": true, "choice": true, "clue": true,
	"free_text": true, "password": true, "photo": true, "pincode": true,
	"quiz": true, "rating": true, "scan": true, "sorting": true,
}

type m20260827_quest struct {
	bun.BaseModel `bun:"table:quests"`
	ID            string `bun:"id,pk"`
	GameStructure string `bun:"game_structure"`
}

type m20260827_location struct {
	bun.BaseModel `bun:"table:locations"`
	ID            string `bun:"id,pk"`
	QuestID       string `bun:"quest_id"`
	Name          string `bun:"name"`
	Slug          string `bun:"slug"`
	MarkerID      string `bun:"marker_id"`
	Points        int    `bun:"points"`
	WhenClause    string `bun:"when_clause"`
}

type m20260827_marker struct {
	bun.BaseModel `bun:"table:markers"`
	Code          string `bun:"code,pk"`
}

type m20260827_block struct {
	bun.BaseModel `bun:"table:blocks"`
	ID            string `bun:"id,pk"`
	OwnerID       string `bun:"owner_id"`
	Type          string `bun:"type"`
	Context       string `bun:"context"`
}

type m20260827_blockInsert struct {
	bun.BaseModel      `bun:"table:blocks"`
	ID                 string          `bun:"id,pk"`
	OwnerID            string          `bun:"owner_id"`
	Type               string          `bun:"type"`
	Context            string          `bun:"context"`
	Data               json.RawMessage `bun:"data,type:jsonb"`
	Ordering           int             `bun:"ordering"`
	Points             int             `bun:"points"`
	ValidationRequired bool            `bun:"validation_required"`
}

type m20260827_objective struct {
	bun.BaseModel `bun:"table:objectives"`
	ID            string    `bun:"id,pk"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
	QuestID       string    `bun:"quest_id"`
	Slug          string    `bun:"slug"`
	Title         string    `bun:"title"`
	WhenClause    string    `bun:"when_clause"`
}

func init() {
	Migrations.MustRegister(m20260827_up, m20260827_down)
}

// m20260827_up converts every quest's Locations (+ Markers + Content/Navigation
// blocks) into equivalent Objectives, leaving the original Location/Marker rows
// untouched. Mapping: a new scan block seeded with the old Marker.Code becomes
// the proof requirement; interactive Content blocks join it in proof (they were
// the actual requirement before); non-interactive Content blocks become reveal
// flavour; Navigation blocks (find-this hints) fold into proof as static
// context; Location.Points moves onto the new scan block, since objectives have
// no points field of their own.
//
// Idempotent per location, per-quest isolated: each quest is its own
// transaction, and within a quest a location is skipped once an objective
// with its slug already exists. This is deliberately not a per-quest skip:
// a quest that partially converted (one location had no marker yet, or the
// run was interrupted) still has its remaining locations picked up on the
// next run, rather than being permanently stuck once the quest holds any
// objective at all. A location that never gets a marker is retried and
// skipped every run, harmlessly, until it does.
//
// LocationIDs stays untouched alongside the new ObjectiveIDs: old data is kept
// on purpose. At the time this migration was written, that left a migrated
// quest transitional and doc-invalid (ExportInstance emitted both Location and
// Objective children, and the doc failed MIXED_LOCATION_OBJECTIVE) until spec
// removal landed. Spec removal has since shipped: Location has no
// representation in the doc at all now, MIXED_LOCATION_OBJECTIVE no longer
// exists, and ExportInstance silently omits any quest's still-unconverted
// LocationIDs rather than failing on them.
func m20260827_up(ctx context.Context, db *bun.DB) error {
	var quests []m20260827_quest
	if err := db.NewSelect().Model(&quests).Scan(ctx); err != nil {
		return fmt.Errorf("loading quests: %w", err)
	}

	var failed []string
	for _, q := range quests {
		if err := m20260827_migrateQuest(ctx, db, q); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", q.ID, err))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf(
			"failed to migrate %d quest(s) (other quests succeeded and were committed): %v",
			len(failed), failed,
		)
	}
	return nil
}

func m20260827_migrateQuest(ctx context.Context, db *bun.DB, q m20260827_quest) error {
	var locations []m20260827_location
	if err := db.NewSelect().Model(&locations).Where("quest_id = ?", q.ID).Scan(ctx); err != nil {
		return fmt.Errorf("loading locations: %w", err)
	}
	if len(locations) == 0 {
		return nil
	}

	// Idempotency is per-location, not per-quest: Objective.Slug is copied
	// verbatim from Location.Slug, so a location already represented by an
	// objective is the one whose slug already appears here. A quest that
	// partially converted (e.g. one location had no marker last run, or the
	// run was interrupted) still has its remaining locations picked up on the
	// next run: a per-quest "any objective exists, skip everything" check
	// would strand those locations unconverted forever.
	//
	// This assumes slug is proof of migration authorship: any objective whose
	// slug matches a location's is treated as that location's converted form.
	// Slug is unique within locations and, separately, within objectives (both
	// DB-enforced per quest), but nothing enforces uniqueness *across* the two
	// tables, so a location and an unrelated, independently-authored objective
	// could in principle collide on slug and cause a false skip here. Accepted
	// because no independent objective authoring exists to collide with.
	var existingObjectives []m20260827_objective
	if err := db.NewSelect().Model(&existingObjectives).Where("quest_id = ?", q.ID).Scan(ctx); err != nil {
		return fmt.Errorf("loading existing objectives: %w", err)
	}
	alreadyMigratedSlug := make(map[string]bool, len(existingObjectives))
	for _, obj := range existingObjectives {
		alreadyMigratedSlug[obj.Slug] = true
	}

	locIDs := make([]string, len(locations))
	for i, loc := range locations {
		locIDs[i] = loc.ID
	}

	var blks []m20260827_block
	if err := db.NewSelect().Model(&blks).Where("owner_id IN (?)", bun.In(locIDs)).Scan(ctx); err != nil {
		return fmt.Errorf("loading blocks: %w", err)
	}
	blocksByOwner := make(map[string][]m20260827_block, len(locations))
	for _, b := range blks {
		blocksByOwner[b.OwnerID] = append(blocksByOwner[b.OwnerID], b)
	}

	markerCodes := make([]string, 0, len(locations))
	for _, loc := range locations {
		if loc.MarkerID != "" {
			markerCodes = append(markerCodes, loc.MarkerID)
		}
	}
	markerExists := make(map[string]bool, len(markerCodes))
	if len(markerCodes) > 0 {
		var markers []m20260827_marker
		if err := db.NewSelect().Model(&markers).Where("code IN (?)", bun.In(markerCodes)).Scan(ctx); err != nil {
			return fmt.Errorf("loading markers: %w", err)
		}
		for _, m := range markers {
			markerExists[m.Code] = true
		}
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(q.GameStructure), &tree); err != nil {
		return fmt.Errorf("decoding game_structure: %w", err)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	objIDByLocID := make(map[string]string, len(locations))
	for _, loc := range locations {
		if alreadyMigratedSlug[loc.Slug] {
			continue
		}
		if !markerExists[loc.MarkerID] {
			// No marker to seed the proof scan code with; skip rather than guess.
			// Retried every run until the location gets a real marker, harmlessly.
			continue
		}

		objID := uuid.New().String()
		now := time.Now()
		obj := &m20260827_objective{
			ID:         objID,
			CreatedAt:  now,
			UpdatedAt:  now,
			QuestID:    q.ID,
			Slug:       loc.Slug,
			Title:      loc.Name,
			WhenClause: loc.WhenClause,
		}
		if _, err := tx.NewInsert().Model(obj).Exec(ctx); err != nil {
			return fmt.Errorf("creating objective for location %s: %w", loc.ID, err)
		}

		for _, b := range blocksByOwner[loc.ID] {
			newContext := "objective_reveal"
			switch {
			case b.Context == "navigation":
				newContext = "objective_proof"
			case b.Context == "location_content" && m20260827_interactiveBlockTypes[b.Type]:
				newContext = "objective_proof"
			}
			if _, err := tx.NewUpdate().
				Model((*m20260827_block)(nil)).
				Set("owner_id = ?", objID).
				Set("context = ?", newContext).
				Where("id = ?", b.ID).
				Exec(ctx); err != nil {
				return fmt.Errorf("re-owning block %s: %w", b.ID, err)
			}
		}

		scanData, err := json.Marshal(map[string]any{
			"prompt": "Scan the code to prove you found it",
			"codes":  []map[string]any{{"value": loc.MarkerID, "generate": false}},
			"match":  "exact",
		})
		if err != nil {
			return fmt.Errorf("encoding scan block data for location %s: %w", loc.ID, err)
		}
		scanBlock := &m20260827_blockInsert{
			ID:                 uuid.New().String(),
			OwnerID:            objID,
			Type:               "scan",
			Context:            "objective_proof",
			Data:               scanData,
			Ordering:           -1, // sorts before any pre-existing moved block
			Points:             loc.Points,
			ValidationRequired: true,
		}
		if _, err := tx.NewInsert().Model(scanBlock).Exec(ctx); err != nil {
			return fmt.Errorf("creating scan block for location %s: %w", loc.ID, err)
		}

		objIDByLocID[loc.ID] = objID
	}

	if len(objIDByLocID) > 0 {
		m20260827_addObjectiveIDsToTree(tree, objIDByLocID)
		newStructure, err := json.Marshal(tree)
		if err != nil {
			return fmt.Errorf("encoding game_structure: %w", err)
		}
		if _, err := tx.NewUpdate().
			Model((*m20260827_quest)(nil)).
			Set("game_structure = ?", string(newStructure)).
			Where("id = ?", q.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("saving game_structure: %w", err)
		}
	}

	return tx.Commit()
}

// m20260827_addObjectiveIDsToTree walks a GameStructure JSON tree (decoded as
// map[string]any) and, for each entry in a node's location_ids that has a
// mapped objective ID, appends that ID to the same node's objective_ids array,
// preserving order. Recurses into sub_groups. Returns whether anything
// changed. Pure and DB-independent, so it is unit-tested directly.
func m20260827_addObjectiveIDsToTree(node map[string]any, objIDByLocID map[string]string) bool {
	changed := false

	if locIDs, ok := node["location_ids"].([]any); ok && len(locIDs) > 0 {
		objIDs, _ := node["objective_ids"].([]any)
		for _, raw := range locIDs {
			locID, ok := raw.(string)
			if !ok {
				continue
			}
			if objID, found := objIDByLocID[locID]; found {
				objIDs = append(objIDs, objID)
				changed = true
			}
		}
		if changed {
			node["objective_ids"] = objIDs
		}
	}

	if subGroups, ok := node["sub_groups"].([]any); ok {
		for _, raw := range subGroups {
			if sub, ok := raw.(map[string]any); ok {
				if m20260827_addObjectiveIDsToTree(sub, objIDByLocID) {
					changed = true
				}
			}
		}
	}

	return changed
}

// m20260827_down deletes every Objective this migration created (identified
// by matching its slug against a still-existing source Location; see below),
// restores the blocks it moved back to that location, and strips just those
// objective IDs out of the quest's structure tree. Location/Marker rows and
// their blocks were never deleted by up, only re-owned, so nothing here
// destroys real content: only the synthetic scan block up created is deleted.
//
// Only slug-matched objectives are touched. An objective with no matching
// location was not created by this migration (e.g. objectives authored
// directly, not via this migration) and is left completely alone: not
// deleted, not restored,
// and its ID stays in the tree. This makes down safe to run even on a quest
// that has grown real objectives since up ran, rather than assuming every
// objective in the quest is this migration's to undo.
//
// Context restoration is a disclosed best effort, not exact: a moved block
// currently in objective_reveal is unambiguously restored to
// location_content (that is the only context reveal blocks ever came from).
// A moved block currently in objective_proof is restored to location_content
// too, even though some of those blocks originally came from navigation
// (up moves every navigation block to proof unconditionally, so proof mixes
// two origins and nothing recorded which is which). No content is lost
// either way; only that one context tag can end up wrong for a
// navigation-sourced block. Given this migration is meant to run once, that
// tradeoff was chosen over adding a permanent
// bookkeeping column just to make a one-time transform's rollback exact.
func m20260827_down(ctx context.Context, db *bun.DB) error {
	var quests []m20260827_quest
	if err := db.NewSelect().Model(&quests).Scan(ctx); err != nil {
		return fmt.Errorf("loading quests: %w", err)
	}

	for _, q := range quests {
		if err := m20260827_revertQuest(ctx, db, q); err != nil {
			return fmt.Errorf("reverting quest %s: %w", q.ID, err)
		}
	}
	return nil
}

func m20260827_revertQuest(ctx context.Context, db *bun.DB, q m20260827_quest) error {
	var objectives []m20260827_objective
	if err := db.NewSelect().Model(&objectives).Where("quest_id = ?", q.ID).Scan(ctx); err != nil {
		return fmt.Errorf("loading objectives: %w", err)
	}
	if len(objectives) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	matchedObjIDs := make(map[string]bool, len(objectives))
	for _, obj := range objectives {
		// Same slug-as-proof-of-authorship assumption as up (see its comment):
		// an objective is treated as this migration's to undo iff its slug
		// matches a surviving location's, since up copies Location.Slug to
		// Objective.Slug verbatim and up never renames or deletes locations.
		// Nothing enforces slug uniqueness across the two tables, only within
		// each, so an independently-authored objective that happens to share a
		// location's slug would be wrongly matched here too. No match means
		// this objective is not this migration's to undo.
		var loc m20260827_location
		err := tx.NewSelect().Model(&loc).
			Where("quest_id = ? AND slug = ?", q.ID, obj.Slug).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("finding source location for objective %q (slug=%q): %w", obj.ID, obj.Slug, err)
		}
		matchedObjIDs[obj.ID] = true

		// The synthetic scan block up created is the only block ever inserted
		// with ordering -1 (every real block creation path in this codebase
		// uses non-negative ordering); delete it outright.
		if _, err := tx.NewDelete().
			Model((*m20260827_block)(nil)).
			Where("owner_id = ? AND ordering = -1", obj.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("deleting synthetic scan block for objective %q: %w", obj.ID, err)
		}

		// Every other block under this objective was moved here by up from
		// loc.ID; restore it. See the disclosed context-restoration tradeoff above.
		if _, err := tx.NewUpdate().
			Model((*m20260827_block)(nil)).
			Set("owner_id = ?", loc.ID).
			Set("context = ?", "location_content").
			Where("owner_id = ?", obj.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("restoring blocks for objective %q to location %q: %w", obj.ID, loc.ID, err)
		}
	}

	if len(matchedObjIDs) == 0 {
		return tx.Commit()
	}

	idList := make([]string, 0, len(matchedObjIDs))
	for id := range matchedObjIDs {
		idList = append(idList, id)
	}
	if _, err := tx.NewDelete().
		Model((*m20260827_objective)(nil)).
		Where("id IN (?)", bun.In(idList)).
		Exec(ctx); err != nil {
		return fmt.Errorf("deleting objectives: %w", err)
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(q.GameStructure), &tree); err != nil {
		return fmt.Errorf("decoding game_structure: %w", err)
	}
	if m20260827_removeObjectiveIDsFromTree(tree, matchedObjIDs) {
		newStructure, err := json.Marshal(tree)
		if err != nil {
			return fmt.Errorf("encoding game_structure: %w", err)
		}
		if _, err := tx.NewUpdate().
			Model((*m20260827_quest)(nil)).
			Set("game_structure = ?", string(newStructure)).
			Where("id = ?", q.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("saving game_structure: %w", err)
		}
	}

	return tx.Commit()
}

// m20260827_removeObjectiveIDsFromTree removes only the given IDs from every
// node's objective_ids array, recursing into sub_groups. Any objective_ids
// entry not in idsToRemove is left in place: those belong to objectives this
// migration did not create and down has no business touching.
func m20260827_removeObjectiveIDsFromTree(node map[string]any, idsToRemove map[string]bool) bool {
	changed := false

	if objIDs, ok := node["objective_ids"].([]any); ok && len(objIDs) > 0 {
		kept := make([]any, 0, len(objIDs))
		for _, raw := range objIDs {
			id, ok := raw.(string)
			if ok && idsToRemove[id] {
				changed = true
				continue
			}
			kept = append(kept, raw)
		}
		if changed {
			node["objective_ids"] = kept
		}
	}

	if subGroups, ok := node["sub_groups"].([]any); ok {
		for _, raw := range subGroups {
			if sub, ok := raw.(map[string]any); ok {
				if m20260827_removeObjectiveIDsFromTree(sub, idsToRemove) {
					changed = true
				}
			}
		}
	}

	return changed
}
