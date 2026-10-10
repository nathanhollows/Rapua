package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// startButtonBlockType is the one block type the linter names directly: the
// start page is useless without it, and no generic registry capability
// expresses that.
const startButtonBlockType = "start_button"

// retiredSecretRouting is no longer a RouteStrategy, but a document written
// before it was retired can still name it, and saying why is more use than
// calling the value unrecognised.
const retiredSecretRouting = "secret"

type LintResult struct {
	Errors   []LintDiag `json:"errors"`   // Must fix before importing
	Warnings []LintDiag `json:"warnings"` // Should fix but won't block import
}

type LintDiag struct {
	Path    string `json:"path"` // e.g. "structure.children[0].objective.proof.blocks[1]".
	Code    string `json:"code"` // e.g. "SLUG_DUPLICATE", "INVALID_CONTEXT"
	Message string `json:"message"`
}

func (r LintResult) IsValid() bool {
	return len(r.Errors) == 0
}

func (r LintResult) HasError(code string) bool {
	for _, e := range r.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}

// HasWarning is HasError for the diagnostics that do not block an import.
func (r LintResult) HasWarning(code string) bool {
	for _, w := range r.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

// Filter returns the diagnostics whose path starts with prefix. Quest-level
// diagnostics are grouped only by where they came from, and "start" or
// "finish" is the whole of that address.
func (r LintResult) Filter(prefix string) LintResult {
	var out LintResult
	for _, e := range r.Errors {
		if strings.HasPrefix(e.Path, prefix) {
			out.Errors = append(out.Errors, e)
		}
	}
	for _, w := range r.Warnings {
		if strings.HasPrefix(w.Path, prefix) {
			out.Warnings = append(out.Warnings, w)
		}
	}
	return out
}

// Lint validates a GameDoc in three layers: schema, semantic, structural.
// registry is used to check valid block types and contexts; pass blocks.Registry().
func Lint(doc *GameDoc, registry BlockRegistry) LintResult {
	l := &linter{doc: doc, registry: registry}
	l.run()
	return l.result
}

type linter struct {
	doc            *GameDoc
	registry       BlockRegistry
	result         LintResult
	slugs          map[string]bool
	objectiveSlugs map[string]bool // slugs of objectives specifically, a subset of slugs.
	blockIDs       map[string]bool
}

func (l *linter) run() {
	l.slugs = make(map[string]bool)
	l.objectiveSlugs = make(map[string]bool)
	l.blockIDs = make(map[string]bool)

	// Layer 1: Schema
	l.checkSchema()

	// Layer 2: Semantic (only if schema passes to avoid noisy errors)
	if len(l.result.Errors) == 0 {
		l.checkSemantic()
	}

	// Layer 3: Structural warnings (always run)
	l.checkStructural()
}

// --- Layer 1: Schema ---

func (l *linter) checkSchema() {
	if l.doc.Rapua != "v8" {
		l.errorf("", "VERSION_MISMATCH", "expected rapua: \"v8\", got %q", l.doc.Rapua)
	}
	if l.doc.Name == "" {
		l.errorf("name", "MISSING_NAME", "game name is required")
	}
	if l.doc.Structure.IsDraft() {
		l.errorf("structure.draft", "ROOT_DRAFT",
			"the root objective is a draft, which takes the whole quest out of play; "+
				"draft the sections beneath it instead")
	}
	l.checkRootIsContainer()
	l.checkObjectiveDoc("structure", l.doc.Structure, 0)
	for i, b := range l.doc.Start {
		l.checkBlockDoc(fmt.Sprintf("start[%d]", i), b, ContextStart)
	}
	for i, b := range l.doc.Finish {
		l.checkBlockDoc(fmt.Sprintf("finish[%d]", i), b, ContextFinish)
	}
}

// maxNestingDepth is a UI sanity cap rather than a technical limit: past this
// the player has more ancestor context to hold than a phone screen can show.
const maxNestingDepth = 4

// checkObjectiveDoc validates one node and recurses into its children. depth is
// the node's own distance from the root, which is depth 0.
func (l *linter) checkObjectiveDoc(path string, obj ObjectiveDoc, depth int) {
	l.checkObjectiveIdentity(path, obj)
	l.checkChildSettings(path, obj)

	if depth > maxNestingDepth {
		l.warnf(path, "NESTING_TOO_DEEP",
			"objective %q is %d levels deep; more than %d is hard to navigate on a phone",
			obj.Slug, depth, maxNestingDepth)
	}

	for i, child := range obj.Children {
		l.checkObjectiveDoc(fmt.Sprintf("%s.children[%d]", path, i), child, depth+1)
	}
}

func (l *linter) checkObjectiveIdentity(path string, obj ObjectiveDoc) {
	if obj.Slug == "" {
		l.errorf(path+".slug", "MISSING_SLUG", "objective slug is required")
	} else if !slugPattern.MatchString(obj.Slug) {
		l.errorf(path+".slug", "SLUG_INVALID_FORMAT",
			"slug %q must contain only lowercase letters, digits, and hyphens (no leading/trailing hyphens)", obj.Slug)
	}
	if obj.Title == "" {
		l.errorf(path+".title", "MISSING_OBJECTIVE_TITLE", "objective title is required")
	}

	l.checkObjectiveContextDoc(path+".proof", obj.Proof, ContextObjectiveProof)
	l.checkObjectiveContextDoc(path+".reveal", obj.Reveal, ContextObjectiveReveal)

	if len(obj.Proof.Blocks) == 0 || l.registry == nil {
		return
	}
	for _, b := range obj.Proof.Blocks {
		typStr, ok := b["type"].(string)
		if ok && l.registry.IsInteractive(typStr) {
			return
		}
	}
	l.errorf(path+".proof", "PROOF_CONTEXT_NO_INTERACTIVE_BLOCK",
		"a non-empty proof context must contain at least one interactive block, or it gates nothing")
}

// checkChildSettings validates everything that only means something to a node
// with children: routing, the completion band, max_next, and the finish label.
func (l *linter) checkChildSettings(path string, obj ObjectiveDoc) {
	// Published children only: the player engine never loads a draft, so a band
	// counting one is a band no run can meet.
	childCount := obj.PublishedChildCount()
	if childCount == 0 {
		// With nothing below it in play, the node behaves as a leaf: its own
		// proof is the whole of its completion. Worth saying when the author
		// wrote children and drafted them all, since the document still looks
		// like a section.
		if len(obj.Children) > 0 {
			l.warnf(path, "ALL_CHILDREN_DRAFT",
				"every one of this objective's %d children is a draft, so it has nothing to route "+
					"or complete and behaves as a leaf", len(obj.Children))
		}
		l.checkLeafSettings(path, obj)
		return
	}

	l.checkRouting(path+".routing", obj.Routing)
	l.checkBandBounds(path, obj, childCount)

	// Every semantic rule reads the filled band, not the literal fields: an
	// omitted bound is a real value, just not one the author wrote.
	band := obj.Band()
	if obj.FinishLabel != "" && band.AutoCompletes() {
		l.warnf(path+".finish_label", "FINISH_LABEL_UNREACHABLE",
			"finish_label is set but this objective auto-completes at %d of %d children, "+
				"so it never shows a finish button", band.Min, childCount)
	}
	if obj.ChildrenMax != nil && *obj.ChildrenMax == 0 {
		// Reads as "no children needed", so the objective completes before the
		// player has seen any of them and closes the whole subtree. max_next
		// nearby does treat 0 as "all of them", which invites exactly this.
		//
		// Only when the author wrote it. A filled band reaching zero because
		// every child is drafted is the case above, and blaming a field nobody
		// set sends them looking for something that is not there.
		l.errorf(path+".children_max", "BAND_COMPLETES_AT_ZERO",
			"children_max is 0, so this objective completes before any of its %d children are reachable; "+
				"omit it to require all of them", childCount)
	}
	if obj.MaxNext > 0 && obj.Routing != RouteStrategyRandomised {
		// Import only: the editor hides the field once routing moves away, so
		// the lint service filters this out (importOnly).
		l.warnf(path+".max_next", "MAX_NEXT_IGNORED",
			"max_next only applies to %q routing", string(RouteStrategyRandomised))
	}
}

// checkBandBounds checks the literal children_min/children_max the author
// wrote, before any default filling: a bound out of range is an authoring
// mistake whether or not the filled band happens to come out sane.
func (l *linter) checkBandBounds(path string, obj ObjectiveDoc, childCount int) {
	for _, bound := range []struct {
		name  string
		value *int
	}{
		{"children_min", obj.ChildrenMin},
		{"children_max", obj.ChildrenMax},
	} {
		if bound.value == nil {
			continue
		}
		if *bound.value < 0 {
			l.errorf(path+"."+bound.name, "BAND_OUT_OF_RANGE",
				"%s (%d) must not be negative", bound.name, *bound.value)
		}
		if *bound.value > childCount {
			l.errorf(path+"."+bound.name, "BAND_OUT_OF_RANGE",
				"%s (%d) exceeds the %d children below this objective, so it can never be met",
				bound.name, *bound.value, childCount)
		}
	}

	if obj.ChildrenMin != nil && obj.ChildrenMax != nil && *obj.ChildrenMin > *obj.ChildrenMax {
		l.errorf(path+".children_min", "BAND_MIN_EXCEEDS_MAX",
			"children_min (%d) exceeds children_max (%d)", *obj.ChildrenMin, *obj.ChildrenMax)
	}
}

// checkLeafSettings warns about fields that govern children on a node with
// none. They are inert rather than wrong, which is why these are warnings: an
// author mid-edit may be about to add the children.
//
// Routing is not among them. Every objective is created with one, so a leaf
// carrying routing is the normal state rather than something an author did,
// and a warning on every leaf of every quest is noise that teaches people to
// ignore the panel. An objective moved out of a section keeps its settings on
// purpose: they are what it needs again the moment it gains children.
func (l *linter) checkLeafSettings(path string, obj ObjectiveDoc) {
	if obj.ChildrenMin != nil || obj.ChildrenMax != nil {
		l.warnf(path+".children_min", "BAND_ON_LEAF",
			"children_min/children_max have no effect on an objective with no children")
	}
	if obj.MaxNext > 0 {
		l.warnf(path+".max_next", "MAX_NEXT_ON_LEAF",
			"max_next has no effect on an objective with no children")
	}
	if obj.FinishLabel != "" {
		l.warnf(path+".finish_label", "FINISH_LABEL_UNREACHABLE",
			"finish_label has no effect on an objective with no children")
	}
}

func (l *linter) checkObjectiveContextDoc(path string, objCtx ObjectiveContextDoc, ctx BlockContext) {
	for i, b := range objCtx.Blocks {
		l.checkBlockDoc(fmt.Sprintf("%s.blocks[%d]", path, i), b, ctx)
	}
}

func (l *linter) checkBlockDoc(path string, b BlockDoc, _ BlockContext) { //nolint:gocognit
	typVal, ok := b["type"]
	if !ok {
		l.errorf(path+".type", "MISSING_BLOCK_TYPE", "block is missing required \"type\" field")
		return
	}
	typStr, ok := typVal.(string)
	if !ok {
		l.errorf(path+".type", "INVALID_BLOCK_TYPE", "block \"type\" must be a string")
		return
	}
	if l.registry != nil && !l.registry.IsValidType(typStr) {
		l.errorf(path+".type", "UNKNOWN_BLOCK_TYPE", "unknown block type %q", typStr)
		return
	}
	if pointsVal, ok := b["points"]; ok {
		switch v := pointsVal.(type) {
		case float64:
			if v < 0 {
				l.errorf(path+".points", "INVALID_POINTS", "block points must be non-negative")
			}
		case json.Number:
			if n, err := v.Float64(); err == nil && n < 0 {
				l.errorf(path+".points", "INVALID_POINTS", "block points must be non-negative")
			}
		}
	}
	if l.registry != nil {
		known := l.registry.KnownFields(typStr)
		if known != nil {
			knownSet := make(map[string]bool, len(known)+3) //nolint:mnd // +3 for promoted fields: type, id, points
			for _, f := range known {
				knownSet[f] = true
			}
			// Promoted fields valid on every block.
			knownSet["type"] = true
			knownSet["id"] = true
			knownSet["points"] = true
			for k := range b {
				if !knownSet[k] {
					l.warnf(path+"."+k, "UNKNOWN_FIELD",
						"block type %q has no field %q; possible typo", typStr, k)
				}
			}
		}
		errs, warns := l.registry.ValidateBlock(typStr, path, b)
		l.result.Errors = append(l.result.Errors, errs...)
		l.result.Warnings = append(l.result.Warnings, warns...)
	}
}

func (l *linter) checkRouting(path string, r RouteStrategy) {
	switch r {
	case RouteStrategyRandomised, RouteStrategyFreeRoam, RouteStrategyOrdered:
		// valid.
	case RouteStrategy(retiredSecretRouting):
		// Named rather than left to the default arm so a document carrying the
		// retired value is told so directly.
		l.errorf(path, "INVALID_ROUTING",
			"routing %q is retired; an objective is reachable by its parent's routing, "+
				"and a scan block in its proof lets players reach it out of order", r)
	case "":
		// Its own arm because an omission is not a typo. The three strategies
		// produce quests that play nothing like one another, so there is no
		// default that could be assumed on the author's behalf.
		l.errorf(path, "INVALID_ROUTING",
			"routing is not set; choose how this objective offers what is inside it: "+
				"%q one at a time in order, %q all at once, or %q a few at a time",
			string(RouteStrategyOrdered), string(RouteStrategyFreeRoam), string(RouteStrategyRandomised))
	default:
		l.errorf(path, "INVALID_ROUTING", "invalid routing value %q", r)
	}
}

// --- Layer 2: Semantic ---

func (l *linter) checkSemantic() {
	l.collectAndCheckSlugs("structure", l.doc.Structure)
	l.checkBlockContexts("start", l.doc.Start, ContextStart)
	l.trackBlockIDs("start", l.doc.Start)
	l.checkBlockContexts("finish", l.doc.Finish, ContextFinish)
	l.trackBlockIDs("finish", l.doc.Finish)
}

// collectAndCheckSlugs walks the tree recording slugs. The root is included:
// it is an ordinary node whose slug can collide like any other. Drafts are
// included too: a slug is taken whether or not the objective is in play, and
// publishing must not be the moment a collision appears.
func (l *linter) collectAndCheckSlugs(path string, obj ObjectiveDoc) {
	if obj.Slug != "" {
		if l.slugs[obj.Slug] {
			l.errorf(path+".slug", "SLUG_DUPLICATE", "duplicate slug %q", obj.Slug)
		}
		l.slugs[obj.Slug] = true
		l.objectiveSlugs[obj.Slug] = true
	}
	l.checkObjectiveContexts(path, obj)

	for i, child := range obj.Children {
		l.collectAndCheckSlugs(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
}

// checkRootIsContainer rejects content on the root.
//
// The root is the quest, not a place in it: it is never rendered as an
// objective, so proof or reveal blocks on it are content no player is ever
// shown. A second introduction belongs to an objective of its own, ordered
// first, where it can be seen, reordered and drafted like anything else.
func (l *linter) checkRootIsContainer() {
	for _, context := range []struct {
		name   string
		blocks []BlockDoc
	}{
		{"proof", l.doc.Structure.Proof.Blocks},
		{"reveal", l.doc.Structure.Reveal.Blocks},
	} {
		if len(context.blocks) == 0 {
			continue
		}
		l.errorf("structure."+context.name, "ROOT_HAS_CONTENT",
			"the root carries %s content, which no player is shown: the root is the quest rather than "+
				"a place in it. Move it to an objective of its own, first among the root's children",
			context.name)
	}
}

func (l *linter) checkObjectiveContexts(path string, obj ObjectiveDoc) {
	l.checkBlockContexts(path+".proof.blocks", obj.Proof.Blocks, ContextObjectiveProof)
	l.checkBlockContexts(path+".reveal.blocks", obj.Reveal.Blocks, ContextObjectiveReveal)
	l.trackBlockIDs(path+".proof.blocks", obj.Proof.Blocks)
	l.trackBlockIDs(path+".reveal.blocks", obj.Reveal.Blocks)
}

func (l *linter) checkBlockContexts(path string, blocks []BlockDoc, ctx BlockContext) {
	if l.registry == nil {
		return
	}
	for i, b := range blocks {
		blockPath := fmt.Sprintf("%s[%d]", path, i)
		typVal, ok := b["type"]
		if !ok {
			continue
		}
		typStr, ok := typVal.(string)
		if !ok {
			continue
		}
		if !l.registry.CanUseInContext(typStr, ctx) {
			l.errorf(blockPath, "INVALID_CONTEXT",
				"block type %q cannot be used in context %q", typStr, ctx)
		}
	}
}

func (l *linter) trackBlockIDs(path string, blocks []BlockDoc) {
	for i, b := range blocks {
		blockPath := fmt.Sprintf("%s[%d]", path, i)
		if idVal, ok := b["id"]; ok {
			if idStr, ok := idVal.(string); ok && idStr != "" {
				if l.blockIDs[idStr] {
					l.errorf(blockPath+".id", "BLOCK_ID_DUPLICATE",
						"duplicate block id %q", idStr)
				}
				l.blockIDs[idStr] = true
			}
		}
	}
}

// --- Layer 3: Structural warnings ---

func (l *linter) checkStructural() {
	hasStartButton := false
	for _, b := range l.doc.Start {
		if t, ok := b["type"].(string); ok && t == startButtonBlockType {
			hasStartButton = true
			break
		}
	}
	if len(l.doc.Start) > 0 && !hasStartButton {
		l.warnf("start", "NO_START_BUTTON",
			"start page has no start_button block; players won't be able to start the game")
	}

	// Points on a quest that has them switched off used to warn. The editor
	// hides every points control while they are off, so the field that would
	// clear the warning cannot be reached from the page the warning is on:
	// the only way to act on it was to turn points on, which is the opposite
	// of what it asked for. Stored points are simply inert until an author
	// switches them back on, and then they are what that author wanted.
}

// --- Helpers ---

func (l *linter) errorf(path, code, format string, args ...any) {
	l.result.Errors = append(l.result.Errors, LintDiag{
		Path:    path,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	})
}

func (l *linter) warnf(path, code, format string, args ...any) {
	l.result.Warnings = append(l.result.Warnings, LintDiag{
		Path:    path,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	})
}

// --- Depends / variable resolution checks ---
