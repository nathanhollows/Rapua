package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

// LintService checks a quest as it currently stands, rather than a document
// somebody is importing.
//
// It exists because the editor is the one path into a quest that lint never
// saw. Every rule was written against a document, so an author who built the
// same mistake by clicking rather than by importing was told nothing, and
// guards had to be reimplemented at individual mutation surfaces to compensate.
// Here the rules stay in one place and the stored tree is fed to them.
type LintService struct {
	questRepo     repositories.QuestRepository
	settingsRepo  repositories.QuestSettingsRepository
	objectiveRepo repositories.ObjectiveRepository
	blockRepo     repositories.BlockRepository
	registry      game.BlockRegistry
}

func NewLintService(
	questRepo repositories.QuestRepository,
	settingsRepo repositories.QuestSettingsRepository,
	objectiveRepo repositories.ObjectiveRepository,
	blockRepo repositories.BlockRepository,
	registry game.BlockRegistry,
) *LintService {
	return &LintService{
		questRepo:     questRepo,
		settingsRepo:  settingsRepo,
		objectiveRepo: objectiveRepo,
		blockRepo:     blockRepo,
		registry:      registry,
	}
}

// QuestLint is a lint run arranged the way the editor reads it: attached to
// the objective each diagnostic is about.
//
// A flat list answers "is this quest sound", which is the question an import
// asks. An author part way through building one is asking "what have I just
// broken, and where", and a message carrying a document path does not answer
// that.
type QuestLint struct {
	// Unavailable is set when lint could not run at all. The zero value of
	// this struct is a clean quest, so without it a failure renders as an
	// all-clear: the one answer nobody has earned.
	Unavailable bool
	// Quest holds what belongs to no single row: the start and finish pages,
	// the settings, and the shape of the tree itself.
	Quest game.LintResult
	// Objectives is keyed by objective ID, and holds only the objectives with
	// something to report.
	Objectives map[string]game.LintResult
	// RootID lets a caller ask for the root's diagnostics without knowing
	// which row the root is. The root has no card in the tree, so what the
	// rules say about it lives in Quest.
	RootID string
}

// ForEditor returns what an objective's own edit page should show.
//
// That is the row's own diagnostics, plus, for the root, the ones about the
// quest's shape: the root has no card in the tree, so its edit page is the
// only surface carrying the controls those are about. The start and finish
// pages have editors of their own and are left to them, since a complaint
// about a missing start button helps nobody on a page with no start blocks.
func (q QuestLint) ForEditor(objectiveID string) game.LintResult {
	own := q.For(objectiveID)
	if objectiveID == "" || objectiveID != q.RootID {
		return own
	}
	structural := q.Quest.Filter(objectivePrefix)
	quest := q.Quest.Filter("quest")
	return game.LintResult{
		Errors:   concat(own.Errors, quest.Errors, structural.Errors),
		Warnings: concat(own.Warnings, quest.Warnings, structural.Warnings),
	}
}

func concat(groups ...[]game.LintDiag) []game.LintDiag {
	var all []game.LintDiag
	for _, group := range groups {
		all = append(all, group...)
	}
	return all
}

// Unreadable reports a lint run that never happened, which is neither clean
// nor dirty.
func (q QuestLint) Unreadable() bool { return q.Unavailable }

// For returns the diagnostics attached to one objective.
func (q QuestLint) For(objectiveID string) game.LintResult {
	return q.Objectives[objectiveID]
}

// All flattens the run back into one result: quest-level first, then each
// objective's in a stable order.
func (q QuestLint) All() game.LintResult {
	all := game.LintResult{
		Errors:   append([]game.LintDiag(nil), q.Quest.Errors...),
		Warnings: append([]game.LintDiag(nil), q.Quest.Warnings...),
	}
	ids := make([]string, 0, len(q.Objectives))
	for id := range q.Objectives {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		all.Errors = append(all.Errors, q.Objectives[id].Errors...)
		all.Warnings = append(all.Warnings, q.Objectives[id].Warnings...)
	}
	return all
}

// IsValid is false for a run that never happened: "nothing was found" and
// "nothing was looked for" must not answer the same question the same way.
func (q QuestLint) IsValid() bool { return !q.Unavailable && q.All().IsValid() }

// HasError and HasWarning report what was found, which is nothing at all when
// the run did not happen. Callers deciding whether to proceed want IsValid.
func (q QuestLint) HasError(code string) bool { return q.All().HasError(code) }

func (q QuestLint) HasWarning(code string) bool { return q.All().HasWarning(code) }

// ObjectivesWithErrors and ObjectivesWithWarnings count rows rather than
// diagnostics: the panel at the top of the builder reports how many places
// need attention, and the badges on those rows say what.
func (q QuestLint) ObjectivesWithErrors() int {
	return q.countObjectives(func(r game.LintResult) bool { return len(r.Errors) > 0 })
}

func (q QuestLint) ObjectivesWithWarnings() int {
	return q.countObjectives(func(r game.LintResult) bool {
		return len(r.Errors) == 0 && len(r.Warnings) > 0
	})
}

func (q QuestLint) countObjectives(match func(game.LintResult) bool) int {
	count := 0
	for _, result := range q.Objectives {
		if match(result) {
			count++
		}
	}
	return count
}

// Structural diagnostics for shapes only rows can be in. A document is a tree
// by construction: it cannot express a quest with two roots, none at all, a row
// hanging off a parent that is not there, or a loop. Storage can hold all four,
// so they are named here rather than in the document's own rules.
const (
	// LintNoRoot means nothing to walk from, so no player sees anything.
	LintNoRoot = "NO_ROOT_OBJECTIVE"
	// LintSeveralRoots means no row is identifiable as the root, and picking
	// one would be a guess.
	LintSeveralRoots = "SEVERAL_ROOT_OBJECTIVES"
	// LintOrphaned means a row names a parent this quest does not have, so no
	// walk from the root reaches it.
	LintOrphaned = "OBJECTIVE_ORPHANED"
	// LintParentCycle means a row is its own ancestor.
	LintParentCycle = "OBJECTIVE_PARENT_CYCLE"
)

// objectivePrefix marks a structural diagnostic as being about one row. The
// document rules address an objective by its position in the tree, which is
// the one thing a broken tree does not have.
const objectivePrefix = "objective:"

// LintQuest reports what is wrong with a quest as stored.
//
// Only the absence of a single root stops the document rules running, because
// those rules walk down from one: with no root there is nothing to start from,
// and with several there is no way to say which. An orphan or a loop among
// unreachable rows leaves that walk intact, and suppressing every other
// diagnostic over one recoverable row would hide the whole quest behind it.
func (s *LintService) LintQuest(ctx context.Context, questID string) (QuestLint, error) {
	objectives, err := s.objectiveRepo.FindTreeByQuestID(ctx, questID)
	if err != nil {
		return QuestLint{}, fmt.Errorf("loading objectives: %w", err)
	}

	// Structural diagnostics address rows by slug, because a tree too broken to
	// walk has no paths to address them by. The objectives_quest_slug index
	// keeps that mapping unique; uniqueSlugs holds the line anyway, since
	// sending an author to fix the wrong row is worse than saying only that
	// something is wrong.
	idBySlug := uniqueSlugs(objectives)

	// The structural diagnostics still name their rows, so an orphan or a loop
	// is marked where it is whether or not a document gets built.
	root, structural := checkStoredTree(objectives)
	if root == nil {
		return groupByObjective(structural, game.ObjectiveDoc{}, idBySlug, ""), nil
	}

	quest, err := s.questRepo.GetByID(ctx, questID)
	if err != nil {
		return QuestLint{}, fmt.Errorf("loading quest: %w", err)
	}
	settings, err := s.settingsRepo.GetByQuestID(ctx, questID)
	if err != nil {
		return QuestLint{}, fmt.Errorf("loading settings: %w", err)
	}

	ownerIDs := make([]string, 0, len(objectives)+1)
	ownerIDs = append(ownerIDs, questID)
	for i := range objectives {
		ownerIDs = append(ownerIDs, objectives[i].ID)
	}
	rawBlocks, err := s.blockRepo.FindModelsByOwnerIDs(ctx, ownerIDs)
	if err != nil {
		return QuestLint{}, fmt.Errorf("loading blocks: %w", err)
	}

	blocksByOwner := make(map[string][]models.Block, len(ownerIDs))
	for _, block := range rawBlocks {
		blocksByOwner[block.OwnerID] = append(blocksByOwner[block.OwnerID], block)
	}
	childrenOf := make(map[string][]models.Objective, len(objectives))
	byIDForDoc := make(map[string]bool, len(objectives))
	for i := range objectives {
		byIDForDoc[objectives[i].ID] = true
	}
	for i := range objectives {
		if objectives[i].ParentID == "" {
			continue
		}
		// A row whose parent is not in this quest is stranded, and the editor
		// draws it at the top level so it can be dragged back. Hanging it off
		// the root for the document's purposes means it and everything under
		// it get the same content rules as anything else: a stranded section
		// with an unmeetable band is still worth saying so about, and the
		// author is looking straight at it.
		parentID := objectives[i].ParentID
		if !byIDForDoc[parentID] {
			parentID = root.ID
		}
		childrenOf[parentID] = append(childrenOf[parentID], objectives[i])
	}

	start, finish := buildStartFinish(blocksByOwner[questID])
	doc := &game.GameDoc{
		Rapua: "v8",
		ID:    questID,
		Name:  quest.Name,
		Settings: game.SettingsDoc{
			EnablePoints:    settings.EnablePoints,
			ShowLeaderboard: settings.ShowLeaderboard,
		},
		Start:     start,
		Finish:    finish,
		Structure: buildObjectiveTree(*root, childrenOf, blocksByOwner),
	}

	result := game.Lint(doc, s.registry)
	result.Errors = append(result.Errors, structural.Errors...)
	result.Warnings = append(result.Warnings, structural.Warnings...)
	return groupByObjective(result, doc.Structure, idBySlug, root.ID), nil
}

// groupByObjective attaches each diagnostic to the row it is about, leaving
// the rest with the quest.
func groupByObjective(
	result game.LintResult, root game.ObjectiveDoc, idBySlug map[string]string, rootID string,
) QuestLint {
	index := objectivePathIndex(root)
	grouped := QuestLint{Objectives: make(map[string]game.LintResult), RootID: rootID}

	for _, diag := range result.Errors {
		id := objectiveIDForPath(diag.Path, index, idBySlug)
		if id == "" {
			grouped.Quest.Errors = append(grouped.Quest.Errors, diag)
			continue
		}
		entry := grouped.Objectives[id]
		entry.Errors = append(entry.Errors, diag)
		grouped.Objectives[id] = entry
	}
	for _, diag := range result.Warnings {
		id := objectiveIDForPath(diag.Path, index, idBySlug)
		if id == "" {
			grouped.Quest.Warnings = append(grouped.Quest.Warnings, diag)
			continue
		}
		entry := grouped.Objectives[id]
		entry.Warnings = append(entry.Warnings, diag)
		grouped.Objectives[id] = entry
	}
	return grouped
}

// uniqueSlugs maps slug to row ID, omitting any slug more than one row claims.
func uniqueSlugs(objectives []models.Objective) map[string]string {
	count := make(map[string]int, len(objectives))
	for i := range objectives {
		count[objectives[i].Slug]++
	}
	byslug := make(map[string]string, len(objectives))
	for i := range objectives {
		if count[objectives[i].Slug] == 1 {
			byslug[objectives[i].Slug] = objectives[i].ID
		}
	}
	return byslug
}

// objectivePathIndex maps each document path that names an objective to that
// objective's ID. It indexes by walking the tree the way the linter walks it,
// so the paths line up without either side knowing about the other.
//
// The ID comes off the document rather than being looked up by slug: a path is
// unique where a slug need not be, and resolving through a slug map sent both
// halves of a duplicate pair to whichever row was written last.
//
// The root is left out on purpose: it is the quest rather than a place in it
// and has no row in the editor, so what the rules say about it has nowhere to
// hang and belongs with the quest.
func objectivePathIndex(root game.ObjectiveDoc) map[string]string {
	index := make(map[string]string)

	var walk func(path string, obj game.ObjectiveDoc)
	walk = func(path string, obj game.ObjectiveDoc) {
		if obj.ID != "" {
			index[path] = obj.ID
		}
		for i, child := range obj.Children {
			walk(fmt.Sprintf("%s.children[%d]", path, i), child)
		}
	}
	for i, child := range root.Children {
		walk(fmt.Sprintf("structure.children[%d]", i), child)
	}
	return index
}

// objectiveIDForPath finds the objective a diagnostic is about by trimming its
// path back to the longest prefix that names one. A path points at the exact
// field being complained about (structure.children[2].proof.blocks[0].type),
// and every step up from there is still inside the same objective until it
// reaches one.
//
// Returns "" for the diagnostics that belong to no row: the start and finish
// pages, the settings, the shape of the tree, and anything about the root.
func objectiveIDForPath(path string, index, idBySlug map[string]string) string {
	if slug, ok := strings.CutPrefix(path, objectivePrefix); ok {
		return idBySlug[slug]
	}
	for path != "" {
		if id, ok := index[path]; ok {
			return id
		}
		cut := strings.LastIndex(path, ".")
		if cut < 0 {
			return ""
		}
		path = path[:cut]
	}
	return ""
}

// checkStoredTree reports the shapes a document cannot be in, and returns the
// root when there is exactly one.
func checkStoredTree(objectives []models.Objective) (*models.Objective, game.LintResult) {
	var result game.LintResult

	byID := make(map[string]models.Objective, len(objectives))
	for _, obj := range objectives {
		byID[obj.ID] = obj
	}

	var roots []models.Objective
	for i := range objectives {
		if objectives[i].ParentID == "" {
			roots = append(roots, objectives[i])
			continue
		}
		if _, ok := byID[objectives[i].ParentID]; !ok {
			result.Errors = append(result.Errors, game.LintDiag{
				Path: "objective:" + objectives[i].Slug,
				Code: LintOrphaned,
				Message: fmt.Sprintf("%q sits under an objective this quest does not have,"+
					" so nothing reaches it", objectives[i].Title),
			})
		}
	}

	switch len(roots) {
	case 0:
		result.Errors = append(result.Errors, game.LintDiag{
			Path:    "quest",
			Code:    LintNoRoot,
			Message: "this quest has no root objective, so there is nothing to show a player",
		})
	case 1:
	default:
		slugs := make([]string, len(roots))
		for i, root := range roots {
			slugs[i] = root.Slug
		}
		sort.Strings(slugs)
		result.Errors = append(result.Errors, game.LintDiag{
			Path: "quest",
			Code: LintSeveralRoots,
			Message: fmt.Sprintf("this quest has %d objectives with no parent (%v),"+
				" so none of them is identifiably the root", len(roots), slugs),
		})
	}

	// Only the rows that close a loop are named. Everything hanging below one
	// is unreachable too, but it is not its own ancestor and its author cannot
	// fix it: blaming twenty descendants buries the two rows that are actually
	// wrong under twenty copies of the wrong message.
	for _, obj := range onCycle(objectives, byID) {
		result.Errors = append(result.Errors, game.LintDiag{
			Path:    objectivePrefix + obj.Slug,
			Code:    LintParentCycle,
			Message: fmt.Sprintf("%q is its own ancestor", obj.Title),
		})
	}

	if len(roots) != 1 {
		return nil, result
	}
	return &roots[0], result
}

// onCycle returns the rows whose parent chain leads back to themselves, in the
// order they were given.
//
// One walk per row would be quadratic, and this runs on every save, so each
// walk memoises what it learned about every row it passed. A row is on a cycle
// when the walk up from it meets that same row again; a row that meets some
// other repeated row is merely below a cycle, which is somebody else's fault.
func onCycle(objectives []models.Objective, byID map[string]models.Objective) []models.Objective {
	const (
		unknown = iota
		cyclic
		grounded
	)
	verdict := make(map[string]int, len(objectives))

	for _, start := range objectives {
		if verdict[start.ID] != unknown {
			continue
		}
		// Where each row sits on this walk, so a repeat can be told apart from
		// a row seen on an earlier one.
		position := make(map[string]int)
		var trail []models.Objective
		current := start

		for {
			if known := verdict[current.ID]; known != unknown {
				// Reached settled ground: nothing further up this walk is on a
				// cycle, whatever the row we joined turned out to be.
				markAll(verdict, trail, grounded)
				break
			}
			if at, repeated := position[current.ID]; repeated {
				// The loop is the tail from the first sighting; everything
				// before it merely hangs below.
				markAll(verdict, trail[:at], grounded)
				markAll(verdict, trail[at:], cyclic)
				break
			}
			position[current.ID] = len(trail)
			trail = append(trail, current)

			parent, ok := byID[current.ParentID]
			if !ok {
				// A row with no parent in this quest: the root, or an orphan.
				// Either way the walk ends without meeting itself.
				markAll(verdict, trail, grounded)
				break
			}
			current = parent
		}
	}

	var cycled []models.Objective
	for _, obj := range objectives {
		if verdict[obj.ID] == cyclic {
			cycled = append(cycled, obj)
		}
	}
	return cycled
}

func markAll(verdict map[string]int, rows []models.Objective, value int) {
	for _, row := range rows {
		verdict[row.ID] = value
	}
}
