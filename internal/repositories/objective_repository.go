package repositories

import (
	"context"
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
	// FindChildrenCount returns how many direct children an objective has.
	FindChildrenCount(ctx context.Context, parentID string) (int, error)
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

func (r *objectiveRepository) FindChildrenCount(ctx context.Context, parentID string) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.Objective)(nil)).
		Where("parent_id = ?", parentID).
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

	oldParentID := objective.ParentID
	objective.ParentID = newParentID
	if _, err := tx.NewUpdate().Model(&objective).
		Column("parent_id").WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("moving objective: %w", err)
	}

	if oldParentID != newParentID {
		if err := renumberSiblings(ctx, tx, oldParentID, "", 0); err != nil {
			return err
		}
	}
	return renumberSiblings(ctx, tx, newParentID, objectiveID, newPosition)
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

// renumberSiblings rewrites one parent's children as 0..n-1. When movedID names
// one of them it is lifted out and reinserted at movedTo first, so the caller's
// index is the one the objective ends up at rather than the one it displaces.
func renumberSiblings(ctx context.Context, tx *bun.Tx, parentID, movedID string, movedTo int) error {
	var siblings []models.Objective
	err := tx.NewSelect().
		Model(&siblings).
		Where("parent_id = ?", parentID).
		Order("position ASC", "id ASC").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("loading siblings: %w", err)
	}

	if movedID != "" {
		siblings = insertAt(siblings, movedID, movedTo)
	}

	for i := range siblings {
		if siblings[i].Position == i {
			continue
		}
		siblings[i].Position = i
		if _, err := tx.NewUpdate().Model(&siblings[i]).
			Column("position").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("renumbering sibling: %w", err)
		}
	}
	return nil
}

// insertAt pulls the named objective out of the ordered slice and puts it back
// at index, clamped to the slice's bounds.
func insertAt(siblings []models.Objective, movedID string, index int) []models.Objective {
	from := -1
	for i := range siblings {
		if siblings[i].ID == movedID {
			from = i
			break
		}
	}
	if from < 0 {
		return siblings
	}

	moved := siblings[from]
	index = max(0, min(index, len(siblings)-1))

	reordered := make([]models.Objective, 0, len(siblings))
	for i, sibling := range siblings {
		if i == from {
			continue
		}
		if len(reordered) == index {
			reordered = append(reordered, moved)
		}
		reordered = append(reordered, sibling)
	}
	if len(reordered) == index {
		reordered = append(reordered, moved)
	}
	return reordered
}

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
