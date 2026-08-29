package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// m20260829_interactiveBlockTypes mirrors m20260827_interactiveBlockTypes: which
// block types require player input, deciding whether a swept Location.Content
// block becomes a proof requirement or reveal flavour. Frozen as of this
// migration's authorship, same reasoning as the original.
var m20260829_interactiveBlockTypes = map[string]bool{
	"broker": true, "checklist": true, "choice": true, "clue": true,
	"free_text": true, "password": true, "photo": true, "pincode": true,
	"quiz": true, "rating": true, "scan": true, "sorting": true,
}

type m20260829_quest struct {
	bun.BaseModel `bun:"table:quests"`
	ID            string `bun:"id,pk"`
	GameStructure string `bun:"game_structure"`
}

type m20260829_location struct {
	bun.BaseModel `bun:"table:locations"`
	ID            string `bun:"id,pk"`
	QuestID       string `bun:"quest_id"`
	Name          string `bun:"name"`
	Slug          string `bun:"slug"`
	MarkerID      string `bun:"marker_id"`
	Points        int    `bun:"points"`
	WhenClause    string `bun:"when_clause"`
}

type m20260829_marker struct {
	bun.BaseModel `bun:"table:markers"`
	Code          string `bun:"code,pk"`
}

type m20260829_block struct {
	bun.BaseModel `bun:"table:blocks"`
	ID            string `bun:"id,pk"`
	OwnerID       string `bun:"owner_id"`
	Type          string `bun:"type"`
	Context       string `bun:"context"`
}

type m20260829_blockInsert struct {
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

type m20260829_objective struct {
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
	Migrations.MustRegister(m20260829_up, m20260829_down)
}

// m20260829_up is the gate before Location can be dropped: it (1) sweeps any
// Location that 20260827_locations_to_objectives.go couldn't convert (it skips,
// and permanently strands, any Location lacking a Marker at the time it ran:
// this picks up any that have acquired one since, using the identical
// conversion logic),
// then (2) strips LocationIDs out of every quest's GameStructure now that
// ObjectiveIDs is guaranteed to hold an equivalent for each one, eliminating the
// "both shown side by side" duplication in the admin UI. If any Location still
// cannot be converted (still no Marker), this fails loudly rather than silently
// proceeding: a subsequent migration drops the locations/markers tables, and
// letting that run over unconverted content would destroy it.
func m20260829_up(ctx context.Context, db *bun.DB) error {
	if !tableExists(ctx, db, "locations") {
		return nil
	}

	var quests []m20260829_quest
	if err := db.NewSelect().Model(&quests).Scan(ctx); err != nil {
		return fmt.Errorf("loading quests: %w", err)
	}

	var stuck []string
	var failed []string
	for _, q := range quests {
		questStuck, err := m20260829_migrateQuest(ctx, db, q)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", q.ID, err))
			continue
		}
		stuck = append(stuck, questStuck...)
	}
	if len(failed) > 0 {
		return fmt.Errorf(
			"failed to migrate %d quest(s) (other quests succeeded and were committed): %v",
			len(failed), failed,
		)
	}
	if len(stuck) > 0 {
		return fmt.Errorf(
			"%d location(s) still cannot be converted to objectives (no marker) and would be "+
				"destroyed by dropping Location next: give them a marker and rerun this migration "+
				"before proceeding: %v",
			len(stuck), stuck,
		)
	}
	return nil
}

// m20260829_migrateQuest converts any of this quest's Locations lacking an
// Objective (same idempotency rule as 20260827: Objective.Slug
// matching Location.Slug is proof of prior conversion), then strips every
// now-converted Location's ID out of the quest's GameStructure. Returns the IDs
// of any Location this pass still could not convert (no marker).
func m20260829_migrateQuest(ctx context.Context, db *bun.DB, q m20260829_quest) ([]string, error) {
	var locations []m20260829_location
	if err := db.NewSelect().Model(&locations).Where("quest_id = ?", q.ID).Scan(ctx); err != nil {
		return nil, fmt.Errorf("loading locations: %w", err)
	}
	if len(locations) == 0 {
		return nil, nil
	}

	var existingObjectives []m20260829_objective
	if err := db.NewSelect().Model(&existingObjectives).Where("quest_id = ?", q.ID).Scan(ctx); err != nil {
		return nil, fmt.Errorf("loading existing objectives: %w", err)
	}
	objIDBySlug := make(map[string]string, len(existingObjectives))
	for _, obj := range existingObjectives {
		objIDBySlug[obj.Slug] = obj.ID
	}

	locIDs := make([]string, len(locations))
	for i, loc := range locations {
		locIDs[i] = loc.ID
	}

	var blks []m20260829_block
	if err := db.NewSelect().Model(&blks).Where("owner_id IN (?)", bun.In(locIDs)).Scan(ctx); err != nil {
		return nil, fmt.Errorf("loading blocks: %w", err)
	}
	blocksByOwner := make(map[string][]m20260829_block, len(locations))
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
		var markers []m20260829_marker
		if err := db.NewSelect().Model(&markers).Where("code IN (?)", bun.In(markerCodes)).Scan(ctx); err != nil {
			return nil, fmt.Errorf("loading markers: %w", err)
		}
		for _, m := range markers {
			markerExists[m.Code] = true
		}
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	// objIDByLocID accumulates BOTH locations converted on a prior run (matched
	// by slug above) and any converted just now, so the tree-stripping pass
	// below removes LocationIDs for all of them, not just this run's new ones.
	objIDByLocID := make(map[string]string, len(locations))
	var stillStuck []string
	for _, loc := range locations {
		if objID, already := objIDBySlug[loc.Slug]; already {
			objIDByLocID[loc.ID] = objID
			continue
		}
		if !markerExists[loc.MarkerID] {
			stillStuck = append(stillStuck, loc.ID)
			continue
		}

		objID := uuid.New().String()
		now := time.Now()
		obj := &m20260829_objective{
			ID:         objID,
			CreatedAt:  now,
			UpdatedAt:  now,
			QuestID:    q.ID,
			Slug:       loc.Slug,
			Title:      loc.Name,
			WhenClause: loc.WhenClause,
		}
		if _, err := tx.NewInsert().Model(obj).Exec(ctx); err != nil {
			return nil, fmt.Errorf("creating objective for location %s: %w", loc.ID, err)
		}

		for _, b := range blocksByOwner[loc.ID] {
			newContext := "objective_reveal"
			switch {
			case b.Context == "navigation":
				newContext = "objective_proof"
			case b.Context == "location_content" && m20260829_interactiveBlockTypes[b.Type]:
				newContext = "objective_proof"
			}
			if _, err := tx.NewUpdate().
				Model((*m20260829_block)(nil)).
				Set("owner_id = ?", objID).
				Set("context = ?", newContext).
				Where("id = ?", b.ID).
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("re-owning block %s: %w", b.ID, err)
			}
		}

		scanData, err := json.Marshal(map[string]any{
			"prompt": "Scan the code to prove you found it",
			"codes":  []map[string]any{{"value": loc.MarkerID, "generate": false}},
			"match":  "exact",
		})
		if err != nil {
			return nil, fmt.Errorf("encoding scan block data for location %s: %w", loc.ID, err)
		}
		scanBlock := &m20260829_blockInsert{
			ID:                 uuid.New().String(),
			OwnerID:            objID,
			Type:               "scan",
			Context:            "objective_proof",
			Data:               scanData,
			Ordering:           -1,
			Points:             loc.Points,
			ValidationRequired: true,
		}
		if _, err := tx.NewInsert().Model(scanBlock).Exec(ctx); err != nil {
			return nil, fmt.Errorf("creating scan block for location %s: %w", loc.ID, err)
		}

		objIDByLocID[loc.ID] = objID
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(q.GameStructure), &tree); err != nil {
		return nil, fmt.Errorf("decoding game_structure: %w", err)
	}
	if m20260829_stripConvertedLocationIDs(tree, objIDByLocID) {
		newStructure, err := json.Marshal(tree)
		if err != nil {
			return nil, fmt.Errorf("encoding game_structure: %w", err)
		}
		if _, err := tx.NewUpdate().
			Model((*m20260829_quest)(nil)).
			Set("game_structure = ?", string(newStructure)).
			Where("id = ?", q.ID).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("saving game_structure: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return stillStuck, nil
}

// m20260829_stripConvertedLocationIDs walks a GameStructure JSON tree and, at
// every node, removes from location_ids any ID present in convertedLocIDs
// (i.e. anything with a real Objective now). IDs with no mapped objective are
// deliberately left in place: that is m20260829_up's signal that this quest is
// not safe to drop Location for. Recurses into sub_groups. Returns whether
// anything changed.
func m20260829_stripConvertedLocationIDs(node map[string]any, convertedLocIDs map[string]string) bool {
	changed := false

	if locIDs, ok := node["location_ids"].([]any); ok && len(locIDs) > 0 {
		kept := make([]any, 0, len(locIDs))
		for _, raw := range locIDs {
			locID, ok := raw.(string)
			if ok {
				if _, converted := convertedLocIDs[locID]; converted {
					changed = true
					continue
				}
			}
			kept = append(kept, raw)
		}
		if changed {
			node["location_ids"] = kept
		}
	}

	if subGroups, ok := node["sub_groups"].([]any); ok {
		for _, raw := range subGroups {
			if sub, ok := raw.(map[string]any); ok {
				if m20260829_stripConvertedLocationIDs(sub, convertedLocIDs) {
					changed = true
				}
			}
		}
	}

	return changed
}

// m20260829_down is a documented no-op: stripping LocationIDs back in would
// require having snapshotted their original per-group placement, which nothing
// records, and there is nothing left to usefully roll back to: the Objectives
// this migration's sweep half creates ARE the content going forward. Matches
// this repo's existing precedent for one-time structural cleanups (see
// 20260329000000_cascade_deletes.go's down).
func m20260829_down(_ context.Context, _ *bun.DB) error {
	return nil
}
