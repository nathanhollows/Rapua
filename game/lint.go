package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

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
	l.checkStructureDoc("structure", l.doc.Structure)
	for i, b := range l.doc.Start {
		l.checkBlockDoc(fmt.Sprintf("start[%d]", i), b, ContextStart)
	}
	for i, b := range l.doc.Finish {
		l.checkBlockDoc(fmt.Sprintf("finish[%d]", i), b, ContextFinish)
	}
}

func (l *linter) checkStructureDoc(path string, s StructureDoc) {
	l.checkRouting(path+".routing", s.Routing)
	l.checkCompletion(path, s.Completion, s.MinimumRequired)
	if s.Completion == CompletionMinimum && s.MinimumRequired > len(s.Children) {
		l.errorf(path+".minimum_required", "MINIMUM_REQUIRED_EXCEEDS_CHILDREN",
			"minimum_required (%d) exceeds number of children (%d); players can never complete this group",
			s.MinimumRequired, len(s.Children))
	}
	for i, child := range s.Children {
		l.checkChildDoc(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
}

func (l *linter) checkGroupDoc(path string, g GroupDoc) {
	if g.Name == "" {
		l.errorf(path+".name", "MISSING_GROUP_NAME", "group name is required")
	}
	l.checkRouting(path+".routing", g.Routing)
	l.checkCompletion(path, g.Completion, g.MinimumRequired)
	if g.Completion == CompletionMinimum && g.MinimumRequired > len(g.Children) {
		l.errorf(path+".minimum_required", "MINIMUM_REQUIRED_EXCEEDS_CHILDREN",
			"minimum_required (%d) exceeds number of children (%d); players can never complete this group",
			g.MinimumRequired, len(g.Children))
	}
	if g.AutoAdvance != nil && g.Completion != CompletionMinimum {
		l.warnf(path+".auto_advance", "AUTO_ADVANCE_IGNORED",
			"auto_advance has no effect unless completion is %q", string(CompletionMinimum))
	}
	for i, child := range g.Children {
		l.checkChildDoc(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
}

func (l *linter) checkChildDoc(path string, child ChildDoc) {
	switch {
	case child.Group != nil:
		l.checkGroupDoc(path+".group", *child.Group)
	case child.Objective != nil:
		l.checkObjectiveDoc(path+".objective", *child.Objective)
	default:
		l.errorf(path, "EMPTY_CHILD", "child has neither group nor objective")
	}
}

// checkObjectiveDoc applies the proof-context composition rule: a non-empty
// proof context must contain at least one interactive block, or it gates nothing.
func (l *linter) checkObjectiveDoc(path string, obj ObjectiveDoc) {
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
	case RouteStrategyRandomised, RouteStrategyFreeRoam, RouteStrategyOrdered, RouteStrategySecret:
		// valid.
	default:
		l.errorf(path, "INVALID_ROUTING", "invalid routing value %q", r)
	}
}

func (l *linter) checkCompletion(path string, c CompletionType, minRequired int) {
	switch c {
	case CompletionAll, CompletionMinimum:
		// valid.
	default:
		l.errorf(path+".completion", "INVALID_COMPLETION", "invalid completion value %q", c)
		return
	}
	if c != CompletionMinimum && minRequired != 0 {
		l.errorf(path+".minimum_required", "MINIMUM_REQUIRED_MISMATCH",
			"minimum_required is only valid when completion is \"minimum\"")
	}
	if c == CompletionMinimum && minRequired <= 0 {
		l.errorf(path+".minimum_required", "MINIMUM_REQUIRED_MISSING",
			"minimum_required must be positive when completion is \"minimum\"")
	}
}

// --- Layer 2: Semantic ---

func (l *linter) checkSemantic() {
	l.collectAndCheckSlugsInChildren("structure", l.doc.Structure.Children)
	l.checkBlockContexts("start", l.doc.Start, ContextStart)
	l.trackBlockIDs("start", l.doc.Start)
	l.checkBlockContexts("finish", l.doc.Finish, ContextFinish)
	l.trackBlockIDs("finish", l.doc.Finish)
	l.checkDependsInDoc()
}

func (l *linter) collectAndCheckSlugsInChildren(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		switch {
		case child.Objective != nil:
			slug := child.Objective.Slug
			if slug != "" {
				if l.slugs[slug] {
					l.errorf(childPath+".objective.slug", "SLUG_DUPLICATE",
						"duplicate slug %q", slug)
				}
				l.slugs[slug] = true
				l.objectiveSlugs[slug] = true
			}
			l.checkObjectiveContexts(childPath+".objective", *child.Objective)
		case child.Group != nil:
			l.collectAndCheckSlugsInChildren(childPath+".group", child.Group.Children)
		}
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
	for i, child := range l.doc.Structure.Children {
		if child.Objective != nil {
			l.warnf(fmt.Sprintf("structure.children[%d].objective", i), "ROOT_OBJECTIVE_HIDDEN",
				"objective %q is a direct child of the root structure and will not be shown; wrap it in a group",
				child.Objective.Title)
		}
	}
	l.checkStructuralChildren("structure", l.doc.Structure.Children)

	hasStartButton := false
	for _, b := range l.doc.Start {
		if t, ok := b["type"].(string); ok && t == "start_button" {
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
		l.warnChildrenBlockPoints("structure", l.doc.Structure.Children)
	}
}

func (l *linter) checkStructuralChildren(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		if child.Group != nil {
			if len(child.Group.Children) == 0 {
				l.warnf(childPath+".group", "EMPTY_GROUP",
					"group %q has no children", child.Group.Name)
			}
			l.checkStructuralChildren(childPath+".group", child.Group.Children)
		}
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

func (l *linter) warnChildrenBlockPoints(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		switch {
		case child.Objective != nil:
			// Objectives have no points field of their own; points are block-level only.
			obj := *child.Objective
			l.warnBlocksWithPoints(childPath+".objective.proof.blocks", obj.Proof.Blocks)
			l.warnBlocksWithPoints(childPath+".objective.reveal.blocks", obj.Reveal.Blocks)
		case child.Group != nil:
			l.warnChildrenBlockPoints(childPath+".group", child.Group.Children)
		}
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
	l.collectVarsFromChildrenIntoSet(l.doc.Structure.Children, l.definedVars)
}

func (l *linter) collectVarsFromBlocks(blocks []BlockDoc) {
	for _, b := range blocks {
		for _, v := range l.blockDocSetsVars(b) {
			l.definedVars[v] = true
		}
	}
}

func (l *linter) collectVarsFromChildrenIntoSet(children []ChildDoc, vars map[string]bool) {
	for _, child := range children {
		switch {
		case child.Objective != nil:
			l.collectVarsFromObjectiveContext(child.Objective.Proof, vars)
			l.collectVarsFromObjectiveContext(child.Objective.Reveal, vars)
		case child.Group != nil:
			l.collectVarsFromChildrenIntoSet(child.Group.Children, vars)
		}
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
	l.checkDependsInChildren("structure", l.doc.Structure.Children)
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

func (l *linter) checkDependsInChildren(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		switch {
		case child.Objective != nil:
			obj := child.Objective
			objPath := childPath + ".objective"
			l.checkDepends(objPath+".depends", obj.Depends)
			l.recordDependsEdges(objPath, obj)
		case child.Group != nil:
			l.checkDependsInChildren(childPath+".group", child.Group.Children)
		}
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
func (l *linter) recordDependsEdges(path string, obj *ObjectiveDoc) {
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
