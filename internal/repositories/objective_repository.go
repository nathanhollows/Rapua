package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// Tree invariants callers may want to distinguish. parent_id carries no foreign
// key, so these are the only thing standing between a mistyped id and a subtree
// that no walk from the root can reach.
var (
	ErrNoRootObjective        = errors.New("quest has no root objective")
	ErrAmbiguousRootObjective = errors.New("quest has more than one objective without a parent")
	ErrCannotMoveRoot         = errors.New("the root objective cannot be moved")
	ErrParentRequired         = errors.New("an objective needs a parent; only the root has none")
	ErrSelfParent             = errors.New("an objective cannot be its own parent")
	ErrParentNotInQuest       = errors.New("parent is not an objective of the same quest")
	ErrParentIsDescendant     = errors.New("an objective cannot move beneath its own descendant")
	ErrParentStranded         = errors.New("parent is not reachable from the root")
)

type ObjectiveRepository interface {
	GetByID(ctx context.Context, objectiveID string) (*models.Objective, error)
	GetByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error)
	// SlugAvailable reports whether a slug is free within a quest, ignoring
	// excludeID so an objective keeps its own slug through a rename. It answers
	// the question rather than returning a not-found error, because "no rows"
	// is a storage detail.
	SlugAvailable(ctx context.Context, questID, slug, excludeID string) (bool, error)
	FindByIDs(ctx context.Context, questID string, objectiveIDs []string) ([]*models.Objective, error)
	FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error)
	// FindTreeByQuestID returns every objective in a quest ordered so a parent
	// always precedes its children, and siblings follow their position. One
	// query: the whole tree is small enough that walking it in memory beats a
	// recursive query, and every caller wants all of it.
	FindTreeByQuestID(ctx context.Context, questID string) ([]models.Objective, error)
	// FindRoot returns the quest's root: the one objective with no parent. It
	// is an error for a quest to hold several, since then no row is
	// identifiable as the root and picking one would be a guess.
	FindRoot(ctx context.Context, questID string) (*models.Objective, error)
	// FindPublishedChildrenCount returns how many direct children an objective
	// has that are in play. Drafts are excluded because the only caller is the
	// completion band, and lint counts the same way: a band that counts a row
	// no run loads is a band no run can meet.
	FindPublishedChildrenCount(ctx context.Context, parentID string) (int, error)
	// FindChildren returns one objective's direct children in position order.
	// It is scoped by quest because parent_id has no foreign key, so an id that
	// leaked in from elsewhere would otherwise pull in another quest's rows.
	FindChildren(ctx context.Context, questID, parentID string) ([]models.Objective, error)
	// Reposition moves an objective under newParentID, ending up at index
	// newPosition among its siblings there. It is the only way to change an
	// objective's place in the tree; UpdateTx deliberately cannot.
	//
	// It refuses to move the root only where the root is identifiable, which
	// means the quest holds exactly one objective with no parent. Where several
	// do, none of them is the root by that test, and a caller that knows which
	// one it means has to protect it: this will accept a move that turns it
	// into somebody's child.
	Reposition(ctx context.Context, tx *bun.Tx, objectiveID, newParentID string, newPosition int) error
	// PlaceTx is the only writer of parent_id and position. Every path that
	// rearranges the tree goes through it, because (parent_id, position) is
	// unique and SQLite checks it per statement: a row cannot take a number
	// another row has not let go of yet, so placements have to be parked clear
	// and settled in two passes rather than written where they belong.
	//
	// Callers must pass every sibling of every parent they touch. A placement
	// settling onto a slot held by a row that was left out is the collision
	// this exists to prevent.
	PlaceTx(ctx context.Context, tx *bun.Tx, placements []Placement) error
	LoadBlocks(ctx context.Context, objective *models.Objective) error
	// Create writes one objective outside a transaction, for callers that are
	// not already in one. It applies the same checks as CreateTx.
	Create(ctx context.Context, objective *models.Objective) error
	CreateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error
	// UpdateTx writes every column of an objective except its place in the
	// tree, so it needs a fully loaded model: a sparsely built one blanks the
	// fields it leaves unset. parent_id and position are excluded because
	// blanking those would not lose a field, it would detach a whole subtree.
	UpdateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error
	Delete(ctx context.Context, tx *bun.Tx, objectiveID string) error
	DeleteByQuestID(ctx context.Context, tx *bun.Tx, questID string) error
}

// Placement is where one objective sits: under a parent, at an index among the
// children placed there.
type Placement struct {
	ObjectiveID string
	ParentID    string
	Position    int
}

type objectiveRepository struct {
	db *bun.DB
}

func NewObjectiveRepository(db *bun.DB) ObjectiveRepository {
	return &objectiveRepository{db: db}
}

func (r *objectiveRepository) GetByID(ctx context.Context, objectiveID string) (*models.Objective, error) {
	var objective models.Objective
	err := r.db.NewSelect().
		Model(&objective).
		Where("id = ?", objectiveID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objective: %w", err)
	}
	return &objective, nil
}

func (r *objectiveRepository) GetByQuestIDAndSlug(
	ctx context.Context,
	questID, slug string,
) (*models.Objective, error) {
	var objective models.Objective
	err := r.db.NewSelect().
		Model(&objective).
		Where("quest_id = ? AND slug = ?", questID, slug).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objective: %w", err)
	}
	return &objective, nil
}

func (r *objectiveRepository) SlugAvailable(
	ctx context.Context,
	questID, slug, excludeID string,
) (bool, error) {
	query := r.db.NewSelect().
		Model((*models.Objective)(nil)).
		Where("quest_id = ? AND slug = ?", questID, slug)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return false, fmt.Errorf("checking slug availability: %w", err)
	}
	return count == 0, nil
}

func (r *objectiveRepository) FindByIDs(
	ctx context.Context, questID string, objectiveIDs []string,
) ([]*models.Objective, error) {
	if len(objectiveIDs) == 0 {
		return []*models.Objective{}, nil
	}

	var objectives []*models.Objective
	err := r.db.NewSelect().
		Model(&objectives).
		Where("quest_id = ?", questID).
		Where("id IN (?)", bun.In(objectiveIDs)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objectives by IDs: %w", err)
	}
	return objectives, nil
}

// FindByQuestID returns a quest's objectives in no particular order, for
// callers that only need to look rows up by ID. Use FindTreeByQuestID where the
// shape of the tree matters.
func (r *objectiveRepository) FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error) {
	var objectives []models.Objective
	err := r.db.NewSelect().
		Model(&objectives).
		Where("quest_id = ?", questID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objectives for quest: %w", err)
	}
	return objectives, nil
}

// FindTreeByQuestID reads the quest's rows in one query and orders them by the
// walk, not by the query: the database sorts siblings by position, and
// sortTreeOrder puts each parent ahead of its children.
func (r *objectiveRepository) FindTreeByQuestID(
	ctx context.Context, questID string,
) ([]models.Objective, error) {
	var objectives []models.Objective
	err := r.db.NewSelect().
		Model(&objectives).
		Where("quest_id = ?", questID).
		Order("position ASC", "id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding objective tree: %w", err)
	}
	return sortTreeOrder(objectives), nil
}

// sortTreeOrder returns objectives with every parent before its children,
// siblings in position order.
//
// Rows the walk from the root cannot reach are appended at the end rather than
// dropped, so a caller sees a damaged tree instead of silently losing part of
// it. That covers both a missing parent and a cycle: the walk starts only from
// the parentless rows, and a cycle contains none, so its rows are never visited
// and the recursion cannot run away.
func sortTreeOrder(objectives []models.Objective) []models.Objective {
	childrenOf := make(map[string][]models.Objective, len(objectives))
	for _, obj := range objectives {
		childrenOf[obj.ParentID] = append(childrenOf[obj.ParentID], obj)
	}

	ordered := make([]models.Objective, 0, len(objectives))
	var walk func(parentID string)
	walk = func(parentID string) {
		for _, obj := range childrenOf[parentID] {
			ordered = append(ordered, obj)
			walk(obj.ID)
		}
	}
	walk("")

	if len(ordered) == len(objectives) {
		return ordered
	}
	placed := make(map[string]bool, len(ordered))
	for _, obj := range ordered {
		placed[obj.ID] = true
	}
	for _, obj := range objectives {
		if !placed[obj.ID] {
			ordered = append(ordered, obj)
		}
	}
	return ordered
}

func (r *objectiveRepository) FindRoot(ctx context.Context, questID string) (*models.Objective, error) {
	unattached, err := findUnattached(ctx, r.db, questID)
	if err != nil {
		return nil, err
	}
	switch len(unattached) {
	case 1:
		return &unattached[0], nil
	case 0:
		return nil, fmt.Errorf("%w: quest %q", ErrNoRootObjective, questID)
	default:
		// Returning any one of them would be a guess dressed as an answer.
		return nil, fmt.Errorf("%w: quest %q has %d", ErrAmbiguousRootObjective, questID, len(unattached))
	}
}

// findUnattached returns a quest's objectives with no parent. Exactly one is
// the healthy case, and that one is the root.
func findUnattached(ctx context.Context, db bun.IDB, questID string) ([]models.Objective, error) {
	var objectives []models.Objective
	err := db.NewSelect().
		Model(&objectives).
		Where("quest_id = ? AND (parent_id IS NULL OR parent_id = '')", questID).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding unparented objectives: %w", err)
	}
	return objectives, nil
}

func (r *objectiveRepository) FindPublishedChildrenCount(ctx context.Context, parentID string) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.Objective)(nil)).
		Where("parent_id = ?", parentID).
		Where("draft = ?", false).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting children: %w", err)
	}
	return count, nil
}

func (r *objectiveRepository) FindChildren(
	ctx context.Context, questID, parentID string,
) ([]models.Objective, error) {
	var objectives []models.Objective
	err := r.db.NewSelect().
		Model(&objectives).
		Where("quest_id = ? AND parent_id = ?", questID, parentID).
		Order("position ASC", "id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding children: %w", err)
	}
	return objectives, nil
}

// Reposition moves an objective and renumbers every affected sibling list, so
// positions stay dense. Density is what lets a caller name a plain index
// without first reading which numbers happen to be in use.
//
// newPosition is the index the objective occupies once the move is done, which
// is what a drag-and-drop caller computes: dropping into the last slot of a
// three-item list is position 2, whether or not the objective was already in
// that list. An index past the end lands the objective last rather than
// failing, since a caller working from a stale list should not lose the move.
func (r *objectiveRepository) Reposition(
	ctx context.Context, tx *bun.Tx, objectiveID, newParentID string, newPosition int,
) error {
	var objective models.Objective
	if err := tx.NewSelect().Model(&objective).Where("id = ?", objectiveID).Scan(ctx); err != nil {
		return fmt.Errorf("loading objective to reposition: %w", err)
	}
	// An empty parent means the objective is not attached to anything, which is
	// true of the root and of a row that has yet to be placed. Only the root is
	// immovable, and it is the root precisely because it is the quest's only
	// unattached row: attaching one of several is how a tree gets built.
	if objective.ParentID == "" {
		unattached, err := findUnattached(ctx, tx, objective.QuestID)
		if err != nil {
			return err
		}
		if len(unattached) == 1 {
			return fmt.Errorf("%w: %q", ErrCannotMoveRoot, objectiveID)
		}
	}
	if err := checkNewParent(ctx, tx, objective, newParentID); err != nil {
		return err
	}

	// Both sibling lists go in one placement call, since a move between parents
	// renumbers the list it leaves as well as the one it joins.
	oldParentID := objective.ParentID
	placements, err := siblingPlacements(ctx, tx, newParentID)
	if err != nil {
		return err
	}
	// A move within one parent finds the row already in the list; a move
	// between parents has to bring it across.
	if oldParentID != newParentID {
		placements = append(placements, Placement{
			ObjectiveID: objectiveID, ParentID: newParentID, Position: len(placements),
		})
	}
	placements = orderPlacements(placements, objectiveID, newPosition)

	if oldParentID != newParentID {
		vacated, vErr := siblingPlacements(ctx, tx, oldParentID)
		if vErr != nil {
			return vErr
		}
		placements = append(placements, withoutObjective(vacated, objectiveID)...)
	}

	return r.PlaceTx(ctx, tx, placements)
}

// orderPlacements moves one entry to an index and renumbers the list densely.
func orderPlacements(placements []Placement, movedID string, movedTo int) []Placement {
	from := -1
	for i := range placements {
		if placements[i].ObjectiveID == movedID {
			from = i
			break
		}
	}
	if from < 0 {
		return placements
	}
	moved := placements[from]
	rest := append(append([]Placement{}, placements[:from]...), placements[from+1:]...)

	if movedTo < 0 {
		movedTo = 0
	}
	if movedTo > len(rest) {
		movedTo = len(rest)
	}
	ordered := append(append(append([]Placement{}, rest[:movedTo]...), moved), rest[movedTo:]...)
	for i := range ordered {
		ordered[i].Position = i
	}
	return ordered
}

// withoutObjective drops one entry and closes the gap it leaves.
func withoutObjective(placements []Placement, objectiveID string) []Placement {
	out := make([]Placement, 0, len(placements))
	for _, placement := range placements {
		if placement.ObjectiveID == objectiveID {
			continue
		}
		placement.Position = len(out)
		out = append(out, placement)
	}
	return out
}

// checkNewParent rejects every move that would corrupt the tree rather than
// rearrange it. Each would strand a subtree outside the walk from the root,
// where nothing that reads the tree by descending from the root can see it.
func checkNewParent(
	ctx context.Context, tx *bun.Tx, objective models.Objective, newParentID string,
) error {
	if newParentID == "" {
		return fmt.Errorf("%w: objective %q", ErrParentRequired, objective.ID)
	}
	if newParentID == objective.ID {
		return fmt.Errorf("%w: objective %q", ErrSelfParent, objective.ID)
	}

	var questObjectives []models.Objective
	err := tx.NewSelect().
		Model(&questObjectives).
		Column("id", "parent_id").
		Where("quest_id = ?", objective.QuestID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("loading quest objectives: %w", err)
	}

	parentOf := make(map[string]string, len(questObjectives))
	for _, obj := range questObjectives {
		parentOf[obj.ID] = obj.ParentID
	}
	if _, ok := parentOf[newParentID]; !ok {
		return fmt.Errorf("%w: %q", ErrParentNotInQuest, newParentID)
	}

	// Walk up from the new parent. Reaching the objective being moved means the
	// move would put a node beneath itself; running out of ancestors without
	// reaching an unparented row means the new parent is itself stranded, and
	// hanging more rows off it strands them too. The step bound only stops the
	// walk spinning on a cycle already in the data, which the same
	// out-of-ancestors check then reports.
	ancestor := parentOf[newParentID]
	for steps := 0; steps <= len(parentOf); steps++ {
		if ancestor == objective.ID {
			return fmt.Errorf("%w: %q beneath %q", ErrParentIsDescendant, objective.ID, newParentID)
		}
		if ancestor == "" {
			return nil
		}
		next, ok := parentOf[ancestor]
		if !ok {
			return fmt.Errorf("%w: %q", ErrParentStranded, newParentID)
		}
		ancestor = next
	}
	return fmt.Errorf("%w: %q", ErrParentStranded, newParentID)
}

// PlaceTx implements ObjectiveRepository.
//
// Two passes. The first parks every row being placed at a number nothing else
// is using, which also applies the parent change; the second settles each onto
// the position it was given. Writing straight to the final position instead
// would collide with whichever row is still sitting there, which is every
// rearrangement that is not a pure append.
func (r *objectiveRepository) PlaceTx(ctx context.Context, tx *bun.Tx, placements []Placement) error {
	for i, placement := range placements {
		if _, err := tx.NewUpdate().
			Model((*models.Objective)(nil)).
			Set("parent_id = ?", sql.NullString{String: placement.ParentID, Valid: placement.ParentID != ""}).
			Set("position = ?", placementParkBase+i).
			Where("id = ?", placement.ObjectiveID).
			Exec(ctx); err != nil {
			return fmt.Errorf("parking objective %s: %w", placement.ObjectiveID, err)
		}
	}

	for _, placement := range placements {
		if _, err := tx.NewUpdate().
			Model((*models.Objective)(nil)).
			Set("position = ?", placement.Position).
			Where("id = ?", placement.ObjectiveID).
			Exec(ctx); err != nil {
			return fmt.Errorf("placing objective %s: %w", placement.ObjectiveID, err)
		}
	}
	return nil
}

// siblingPlacements returns one parent's children as a dense placement list in
// their current order.
func siblingPlacements(ctx context.Context, tx *bun.Tx, parentID string) ([]Placement, error) {
	var siblings []models.Objective
	err := tx.NewSelect().
		Model(&siblings).
		Column("id", "position").
		Where("parent_id = ?", parentID).
		Order("position ASC", "id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading siblings: %w", err)
	}

	placements := make([]Placement, len(siblings))
	for i := range siblings {
		placements[i] = Placement{ObjectiveID: siblings[i].ID, ParentID: parentID, Position: i}
	}
	return placements, nil
}

// placementParkBase is where rows wait between the two passes of a placement.
// Every parked row gets a distinct number from this range regardless of parent,
// so nothing collides while the tree is half-written, and it sits far above any
// position a real sibling list reaches.
const placementParkBase = 1_000_000

func (r *objectiveRepository) LoadBlocks(ctx context.Context, objective *models.Objective) error {
	err := r.db.NewSelect().
		Model(objective).
		WherePK().
		Relation("Blocks").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("loading blocks for objective: %w", err)
	}
	return nil
}

func (r *objectiveRepository) Create(ctx context.Context, objective *models.Objective) error {
	return createObjective(ctx, r.db, objective)
}

func (r *objectiveRepository) CreateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error {
	return createObjective(ctx, tx, objective)
}

func createObjective(ctx context.Context, db bun.IDB, objective *models.Objective) error {
	if objective.ID == "" {
		objective.ID = uuid.New().String()
	}

	// An objective naming a parent must name a real one in the same quest,
	// since parent_id has no foreign key to catch a bad id. An objective naming
	// none is unattached, which is a legitimate state: rows exist before
	// anything arranges them into a tree.
	if objective.ParentID != "" {
		if err := checkParentExists(ctx, db, *objective, objective.ParentID); err != nil {
			return err
		}
		// Placement is Reposition's job, so a new child goes last and any
		// Position on the model is ignored. Appending is what keeps positions
		// dense without the caller having to read the siblings first.
		siblings, err := countChildren(ctx, db, objective.QuestID, objective.ParentID)
		if err != nil {
			return err
		}
		objective.Position = siblings
	}

	_, err := db.NewInsert().Model(objective).Exec(ctx)
	return err
}

func countChildren(ctx context.Context, db bun.IDB, questID, parentID string) (int, error) {
	count, err := db.NewSelect().
		Model((*models.Objective)(nil)).
		Where("quest_id = ? AND parent_id = ?", questID, parentID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting children: %w", err)
	}
	return count, nil
}

// checkParentExists rejects a parent that is not an objective of the same
// quest. parent_id has no foreign key, so nothing else catches it.
func checkParentExists(ctx context.Context, db bun.IDB, objective models.Objective, parentID string) error {
	exists, err := db.NewSelect().
		Model((*models.Objective)(nil)).
		Where("id = ? AND quest_id = ?", parentID, objective.QuestID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("checking parent: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: %q", ErrParentNotInQuest, parentID)
	}
	return nil
}

func (r *objectiveRepository) UpdateTx(ctx context.Context, tx *bun.Tx, objective *models.Objective) error {
	_, err := tx.NewUpdate().
		Model(objective).
		ExcludeColumn("parent_id", "position").
		WherePK().
		Exec(ctx)
	return err
}

func (r *objectiveRepository) Delete(ctx context.Context, tx *bun.Tx, objectiveID string) error {
	_, err := tx.NewDelete().Model(&models.Objective{ID: objectiveID}).WherePK().Exec(ctx)
	return err
}

func (r *objectiveRepository) DeleteByQuestID(ctx context.Context, tx *bun.Tx, questID string) error {
	_, err := tx.NewDelete().Model(&models.Objective{}).Where("quest_id = ?", questID).Exec(ctx)
	return err
}
