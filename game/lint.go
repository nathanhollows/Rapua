package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// startButtonBlockType is the one block type the linter names directly: the
// start page is useless without it, and no generic registry capability
// expresses that.
const startButtonBlockType = "start_button"

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
	definedVars    map[string]bool     // all variable names set by any block in the doc.
	usedVars       map[string]bool     // all variable names referenced in any depends list.
	dependsEdges   map[string][]string // objective slug -> slugs its depends list names.
	dependsPaths   map[string]string   // objective slug -> its document path, for diagnostics.
}

func (l *linter) run() {
	l.slugs = make(map[string]bool)
	l.objectiveSlugs = make(map[string]bool)
	l.blockIDs = make(map[string]bool)
	l.definedVars = make(map[string]bool)
	l.usedVars = make(map[string]bool)
	l.dependsEdges = make(map[string][]string)
	l.dependsPaths = make(map[string]string)
	l.collectAllDefinedVars()

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
	childCount := len(obj.Children)
	if childCount == 0 {
		l.checkLeafSettings(path, obj)
		return
	}

	l.checkRouting(path+".routing", obj.Routing)
	l.checkBandBounds(path, obj, childCount)

	// Blocks belong to an objective row, and an objective with children is kept
	// as a group, which is not one. An error rather than a warning because a
	// warning still imports, and a gate that imports without its blocks is a
	// gate the author believes in and the game does not have.
	if len(obj.Proof.Blocks) > 0 || len(obj.Reveal.Blocks) > 0 {
		l.errorf(path+".proof", "SECTION_BLOCKS_NOT_STORED",
			"an objective with children cannot carry its own proof or reveal blocks; "+
				"move them to a child objective")
	}

	// Every semantic rule reads the filled band, not the literal fields: an
	// omitted bound is a real value, just not one the author wrote.
	band := obj.Band()
	if obj.FinishLabel != "" && band.AutoCompletes() {
		l.warnf(path+".finish_label", "FINISH_LABEL_UNREACHABLE",
			"finish_label is set but this objective auto-completes at %d of %d children, "+
				"so it never shows a finish button", band.Min, childCount)
	}
	if band.Max == 0 {
		// Reads as "no children needed", so the objective completes before the
		// player has seen any of them and closes the whole subtree. max_next
		// nearby does treat 0 as "all of them", which invites exactly this.
		l.errorf(path+".children_max", "BAND_COMPLETES_AT_ZERO",
			"children_max is 0, so this objective completes before any of its %d children are reachable; "+
				"omit it to require all of them", childCount)
	}
	if obj.MaxNext > 0 && obj.Routing != RouteStrategyRandomised {
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
func (l *linter) checkLeafSettings(path string, obj ObjectiveDoc) {
	if obj.Routing != "" {
		l.warnf(path+".routing", "ROUTING_ON_LEAF",
			"routing has no effect on an objective with no children")
	}
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
	for _, name := range objCtx.Sets {
		l.checkReservedVarName(path+".sets", name)
	}
}

func (l *linter) checkBlockDoc(path string, b BlockDoc, _ BlockContext) { //nolint:gocognit
	// Runs before the type checks below so that a malformed "sets" still gets
	// its own diagnostic on a block whose type is missing or unknown.
	l.checkSetsShape(path, b)

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
	// Check registry-sourced set vars for reserved namespaces (e.g. choice
	// block options[*].sets). Direct b["sets"] is handled by checkSetsShape above.
	if l.registry != nil {
		l.checkRegistrySetsReserved(typStr, b, path)
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
			// Promoted fields always valid on every block; sets handled below.
			knownSet["type"] = true
			knownSet["id"] = true
			knownSet["points"] = true
			knownSet["sets"] = true
			for k := range b {
				if !knownSet[k] {
					l.warnf(path+"."+k, "UNKNOWN_FIELD",
						"block type %q has no field %q; possible typo", typStr, k)
				}
			}
		}
		// sets is only valid on interactive blocks.
		if _, hasSets := b["sets"]; hasSets && !l.registry.IsInteractive(typStr) {
			l.warnf(
				path+".sets",
				"SETS_ON_CONTENT_BLOCK",
				"block type %q does not support \"sets\"; only interactive blocks (quiz, password, pincode, etc.) may set variables",
				typStr,
			)
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
	case RouteStrategySecret:
		// Named rather than left to the default arm so a document carrying the
		// retired value is told so directly.
		l.errorf(path, "INVALID_ROUTING",
			"routing %q is retired; an objective is reachable by its parent's routing and its depends, "+
				"and a scan block in its proof lets players reach it out of order", r)
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
	l.checkDependsInDoc()
}

// collectAndCheckSlugs walks the tree recording slugs. The root is included:
// it is an ordinary node whose slug can collide like any other.
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

	if !l.doc.Settings.EnablePoints {
		l.warnBlocksWithPoints("start", l.doc.Start)
		l.warnBlocksWithPoints("finish", l.doc.Finish)
		l.warnTreeBlockPoints("structure", l.doc.Structure)
	}
}

func (l *linter) warnBlocksWithPoints(path string, blocks []BlockDoc) {
	for i, b := range blocks {
		if pointsVal, ok := b["points"]; ok {
			var pts float64
			switch v := pointsVal.(type) {
			case float64:
				pts = v
			case json.Number:
				pts, _ = v.Float64()
			}
			if pts > 0 {
				l.warnf(fmt.Sprintf("%s[%d].points", path, i), "POINTS_DISABLED",
					"block has points but enable_points is false in settings")
			}
		}
	}
}

// warnTreeBlockPoints walks the tree. Objectives have no points field of their
// own; points are block-level only.
func (l *linter) warnTreeBlockPoints(path string, obj ObjectiveDoc) {
	l.warnBlocksWithPoints(path+".proof.blocks", obj.Proof.Blocks)
	l.warnBlocksWithPoints(path+".reveal.blocks", obj.Reveal.Blocks)
	for i, child := range obj.Children {
		l.warnTreeBlockPoints(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
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

// collectAllDefinedVars records every var any block can set (via "sets").
// Must run before semantic checks.
func (l *linter) collectAllDefinedVars() {
	l.collectVarsFromBlocks(l.doc.Start)
	l.collectVarsFromBlocks(l.doc.Finish)
	l.collectVarsFromTree(l.doc.Structure, l.definedVars)
}

func (l *linter) collectVarsFromBlocks(blocks []BlockDoc) {
	for _, b := range blocks {
		for _, v := range l.blockDocSetsVars(b) {
			l.definedVars[v] = true
		}
	}
}

func (l *linter) collectVarsFromTree(obj ObjectiveDoc, vars map[string]bool) {
	l.collectVarsFromObjectiveContext(obj.Proof, vars)
	l.collectVarsFromObjectiveContext(obj.Reveal, vars)
	for _, child := range obj.Children {
		l.collectVarsFromTree(child, vars)
	}
}

// collectVarsFromObjectiveContext records vars set by an objective context's
// blocks and by the context's own Sets field, which fires directly (not via a
// block) when every block in the context completes.
func (l *linter) collectVarsFromObjectiveContext(objCtx ObjectiveContextDoc, vars map[string]bool) {
	for _, v := range l.objectiveContextSelfVars(objCtx) {
		vars[v] = true
	}
}

func (l *linter) blockDocsSetsVars(blocks []BlockDoc) []string {
	var vars []string
	for _, b := range blocks {
		vars = append(vars, l.blockDocSetsVars(b)...)
	}
	return vars
}

// objectiveContextSelfVars returns every var name an objective context defines:
// its blocks' sets and the context's own Sets field.
func (l *linter) objectiveContextSelfVars(objCtx ObjectiveContextDoc) []string {
	return append(l.blockDocsSetsVars(objCtx.Blocks), objCtx.Sets...)
}

func (l *linter) checkDependsInDoc() {
	l.checkDependsInTree("structure", l.doc.Structure)
	l.checkDependsCycles()
	l.checkUnusedVars()
}

func (l *linter) checkUnusedVars() {
	for varName := range l.definedVars {
		if !l.usedVars[varName] {
			l.warnf("", "UNUSED_VAR",
				"variable %q is set by a block but never referenced in any depends list", varName)
		}
	}
}

func (l *linter) checkDependsInTree(path string, obj ObjectiveDoc) {
	l.checkDepends(path+".depends", obj.Depends)
	l.recordDependsEdges(path, obj)
	for i, child := range obj.Children {
		l.checkDependsInTree(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
}

func (l *linter) checkDepends(path string, deps DependsField) {
	for i, entry := range deps {
		name, _ := ParseDependsName(entry)
		if name == "" {
			l.errorf(fmt.Sprintf("%s[%d]", path, i), "DEPENDS_EMPTY_NAME",
				"depends entry %q names no variable", entry)
			continue
		}
		l.usedVars[name] = true
		l.checkVarReference(fmt.Sprintf("%s[%d]", path, i), name)
	}
}

// recordDependsEdges stores the objective.<slug> references an objective makes,
// for the cycle check once the whole document has been walked. Negation is
// irrelevant here: "not other" still cannot be evaluated until other is, so it
// is the same edge for reachability purposes.
func (l *linter) recordDependsEdges(path string, obj ObjectiveDoc) {
	if obj.Slug == "" {
		return
	}
	l.dependsPaths[obj.Slug] = path
	for _, entry := range obj.Depends {
		name, _ := ParseDependsName(entry)
		if slug, ok := strings.CutPrefix(name, objectiveVarPrefix); ok && slug != "" {
			l.dependsEdges[obj.Slug] = append(l.dependsEdges[obj.Slug], slug)
		}
	}
}

// checkDependsCycles reports objectives that can never be reached because their
// depends chain leads back to themselves. The self-reference case (an objective
// naming its own slug) is just the one-node cycle.
//
// Only objective.<slug> edges are in this graph. Ordered-sibling edges are not:
// sibling order is a property of the tree, which this grammar does not
// express, so a cycle that only closes through sibling ordering is not caught
// here. Depth-first search over a human-authored document is fast enough that
// nothing here needs to be cleverer than it looks.
func (l *linter) checkDependsCycles() {
	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(l.dependsEdges))

	// Sorted so a document with several cycles reports them in a stable order
	// rather than whatever the map iteration happens to produce.
	roots := make([]string, 0, len(l.dependsEdges))
	for slug := range l.dependsEdges {
		roots = append(roots, slug)
	}
	sort.Strings(roots)

	var walk func(slug string, trail []string)
	walk = func(slug string, trail []string) {
		switch state[slug] {
		case onStack:
			l.errorf(l.dependsPaths[slug]+".depends", "DEPENDS_CYCLE",
				"objective %q can never be reached: its depends chain leads back to itself (%s)",
				slug, strings.Join(append(trail, slug), " -> "))
			return
		case done:
			return
		}
		state[slug] = onStack
		targets := append([]string(nil), l.dependsEdges[slug]...)
		sort.Strings(targets)
		for _, target := range targets {
			walk(target, append(trail, slug))
		}
		state[slug] = done
	}

	for _, slug := range roots {
		walk(slug, nil)
	}
}

// checkVarReference validates a single depends variable reference. An
// objective.<slug> reference is checked against known objective slugs
// specifically: isBuiltInVar accepts any non-empty suffix, so without this a
// typo'd slug would silently never match at runtime instead of being caught here.
func (l *linter) checkVarReference(path, varName string) {
	if slug, ok := strings.CutPrefix(varName, objectiveVarPrefix); ok && slug != "" {
		if !l.objectiveSlugs[slug] {
			l.warnf(path, "UNDEFINED_OBJECTIVE_VAR",
				"depends references objective %q, which does not exist in this game", slug)
		}
		return
	}
	if !l.definedVars[varName] && !isBuiltInVar(varName) {
		l.warnf(path, "UNDEFINED_VAR",
			"depends references variable %q which is never set by any block in this game", varName)
	}
}

// blockDocSetsVars reads from the standard top-level "sets" map and, via the
// registry, from block-type-specific sub-fields (e.g. options[*].sets).
func (l *linter) blockDocSetsVars(b BlockDoc) []string {
	vars := collectSetsFromBlockDoc(b)
	if l.registry != nil {
		if t, ok := b["type"].(string); ok {
			vars = append(vars, l.registry.DocSetsVars(t, b)...)
		}
	}
	return vars
}

// checkSetsShape validates the "sets" field on a block: it must be a list of
// variable names and must not write into reserved runtime namespaces.
func (l *linter) checkSetsShape(path string, b BlockDoc) {
	raw, ok := b["sets"]
	if !ok {
		return
	}
	names, ok := setsNames(raw)
	if !ok {
		l.errorf(path+".sets", "SETS_NOT_LIST",
			`"sets" must be a list of variable names`)
		return
	}
	for _, name := range names {
		l.checkReservedVarName(path+".sets", name)
	}
}

// checkRegistrySetsReserved checks registry-sourced set var names for reserved
// namespaces (e.g. choice block options[*].sets). Direct b["sets"] is handled
// by checkSetsShape. The path points to the block itself because DocSetsVars
// returns bare names with no sub-path index.
func (l *linter) checkRegistrySetsReserved(typStr string, b BlockDoc, path string) {
	for _, name := range l.registry.DocSetsVars(typStr, b) {
		l.checkReservedVarName(path, name)
	}
}

func (l *linter) checkReservedVarName(path string, name string) {
	if IsReservedVarName(name) {
		l.errorf(path, "SETS_RESERVED_NAMESPACE",
			`cannot write to reserved namespace: %q; this var is set automatically by the runtime`, name)
	}
}

// collectSetsFromBlockDoc: malformed "sets" shapes are reported by
// checkSetsShape and contribute no vars.
func collectSetsFromBlockDoc(b BlockDoc) []string {
	raw, ok := b["sets"]
	if !ok {
		return nil
	}
	names, ok := setsNames(raw)
	if !ok {
		return nil
	}
	var vars []string
	for _, name := range names {
		if name != "" {
			vars = append(vars, name)
		}
	}
	return vars
}

// setsNames reads a block doc's "sets" value, which arrives either as []any
// from a JSON decode or as []string when a doc is built in Go. Reports false
// for any other shape, including a list holding a non-string element.
func setsNames(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return v, true
	case []any:
		names := make([]string, 0, len(v))
		for _, elem := range v {
			name, ok := elem.(string)
			if !ok {
				return nil, false
			}
			names = append(names, name)
		}
		return names, true
	}
	return nil, false
}

// isBuiltInVar reports whether name is a built-in variable provided by the
// runtime (not set by any block in the game doc). Referencing a built-in in a
// depends list is valid even though it never appears in definedVars.
//
// objective.<slug> is the only built-in namespace: conditions are truthy-only,
// so the numeric built-ins that only comparisons could read are gone.
//
// checkVarReference validates objective.<slug> against real objective slugs
// before ever consulting this function, so its objective.* branch is presently
// unreachable from that caller. Kept as the canonical definition of the
// built-in namespace shape (see TestIsBuiltInVar_CanonicalSet) rather than
// narrowed to what one caller currently needs.
func isBuiltInVar(name string) bool {
	after, ok := strings.CutPrefix(name, objectiveVarPrefix)
	return ok && len(after) > 0
}

// objectiveVarPrefix is the runtime-owned namespace. Blocks must not write to
// it (the runtime sets it automatically), and depends entries read it to gate
// on another objective's completion.
const objectiveVarPrefix = "objective."

// IsReservedVarName guards the runtime-owned namespace: a block that sets or
// triggers such a var is rejected.
func IsReservedVarName(name string) bool {
	after, ok := strings.CutPrefix(name, objectiveVarPrefix)
	return ok && len(after) > 0
}
