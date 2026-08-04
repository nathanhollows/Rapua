package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// LintResult holds errors and warnings from linting a GameDoc.
type LintResult struct {
	Errors   []LintDiag `json:"errors"`   // Must fix before importing
	Warnings []LintDiag `json:"warnings"` // Should fix but won't block import
}

// LintDiag is a single diagnostic message with a path and code.
type LintDiag struct {
	Path    string `json:"path"` // e.g. "structure.children[0].location.content[1]"
	Code    string `json:"code"` // e.g. "SLUG_DUPLICATE", "INVALID_CONTEXT"
	Message string `json:"message"`
}

// IsValid returns true if there are no errors (warnings are acceptable).
func (r LintResult) IsValid() bool {
	return len(r.Errors) == 0
}

// HasError returns true if any error in the result has the given code.
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
	doc         *GameDoc
	registry    BlockRegistry
	result      LintResult
	slugs       map[string]bool
	blockIDs    map[string]bool
	definedVars map[string]bool // all variable names set by any block in the doc
	usedVars    map[string]bool // all variable names referenced in any when clause
}

func (l *linter) run() {
	l.slugs = make(map[string]bool)
	l.blockIDs = make(map[string]bool)
	l.definedVars = make(map[string]bool)
	l.usedVars = make(map[string]bool)
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
	if l.doc.Rapua != "v7" {
		l.errorf("", "VERSION_MISMATCH", "expected rapua: \"v7\", got %q", l.doc.Rapua)
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
	case child.Location != nil:
		l.checkLocationDoc(path+".location", *child.Location)
	case child.Group != nil:
		l.checkGroupDoc(path+".group", *child.Group)
	default:
		l.errorf(path, "EMPTY_CHILD", "child has neither location nor group")
	}
}

func (l *linter) checkLocationDoc(path string, loc LocationDoc) {
	if loc.Slug == "" {
		l.errorf(path+".slug", "MISSING_SLUG", "location slug is required")
	} else if !slugPattern.MatchString(loc.Slug) {
		l.errorf(path+".slug", "SLUG_INVALID_FORMAT",
			"slug %q must contain only lowercase letters, digits, and hyphens (no leading/trailing hyphens)", loc.Slug)
	}
	if loc.Name == "" {
		l.errorf(path+".name", "MISSING_LOCATION_NAME", "location name is required")
	}
	if loc.Points < 0 {
		l.errorf(path+".points", "INVALID_POINTS", "points must be non-negative")
	}
	if loc.Marker != nil {
		if loc.Marker.Lat == 0 && loc.Marker.Lng == 0 {
			l.warnf(
				path+".marker",
				"ZERO_COORDINATES",
				"marker has zero coordinates; omit marker if location has no map pin",
			)
		}
	}
	for i, b := range loc.Content {
		l.checkBlockDoc(fmt.Sprintf("%s.content[%d]", path, i), b, ContextLocationContent)
	}
	for i, b := range loc.Navigation {
		l.checkBlockDoc(fmt.Sprintf("%s.navigation[%d]", path, i), b, ContextNavigation)
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
	// Warn about unrecognised field names using the block's spec.
	if l.registry != nil {
		known := l.registry.KnownFields(typStr)
		if known != nil {
			knownSet := make(map[string]bool, len(known)+3) //nolint:mnd // +3 for promoted fields: type, id, points
			for _, f := range known {
				knownSet[f] = true
			}
			// Promoted fields always valid on every block; sets/when handled below.
			knownSet["type"] = true
			knownSet["id"] = true
			knownSet["points"] = true
			knownSet["when"] = true
			knownSet["sets"] = true
			for k := range b {
				if !knownSet[k] {
					l.warnf(path+"."+k, "UNKNOWN_FIELD",
						"block type %q has no field %q; possible typo", typStr, k)
				}
			}
		}
		// sets is only valid on interactive blocks (those that require player input).
		if _, hasSets := b["sets"]; hasSets && !l.registry.IsInteractive(typStr) {
			l.warnf(
				path+".sets",
				"SETS_ON_CONTENT_BLOCK",
				"block type %q does not support \"sets\"; only interactive blocks (quiz, password, pincode, etc.) may set variables",
				typStr,
			)
		}
		// Block-type-specific structural validation.
		errs, warns := l.registry.ValidateBlock(typStr, path, b)
		l.result.Errors = append(l.result.Errors, errs...)
		l.result.Warnings = append(l.result.Warnings, warns...)
	}
}

func (l *linter) checkRouting(path string, r RouteStrategy) {
	switch r {
	case RouteStrategyRandomised, RouteStrategyFreeRoam, RouteStrategyOrdered, RouteStrategySecret:
		// valid
	default:
		l.errorf(path, "INVALID_ROUTING", "invalid routing value %q", r)
	}
}

func (l *linter) checkCompletion(path string, c CompletionType, minRequired int) {
	switch c {
	case CompletionAll, CompletionMinimum:
		// valid
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
	l.checkWhenClausesInDoc()
}

func (l *linter) collectAndCheckSlugsInChildren(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		if child.Location != nil {
			slug := child.Location.Slug
			if slug != "" {
				if l.slugs[slug] {
					l.errorf(childPath+".location.slug", "SLUG_DUPLICATE",
						"duplicate slug %q", slug)
				}
				l.slugs[slug] = true
			}
			l.checkLocationContexts(childPath+".location", *child.Location)
		} else if child.Group != nil {
			l.collectAndCheckSlugsInChildren(childPath+".group", child.Group.Children)
		}
	}
}

func (l *linter) checkLocationContexts(path string, loc LocationDoc) {
	l.checkBlockContexts(path+".content", loc.Content, ContextLocationContent)
	l.checkBlockContexts(path+".navigation", loc.Navigation, ContextNavigation)
	l.trackBlockIDs(path+".content", loc.Content)
	l.trackBlockIDs(path+".navigation", loc.Navigation)
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
		if child.Location != nil {
			l.warnf(fmt.Sprintf("structure.children[%d].location", i), "ROOT_LOCATION_HIDDEN",
				"location %q is a direct child of the root structure and will not be shown; wrap it in a group",
				child.Location.Name)
		}
	}
	l.checkStructuralChildren("structure", l.doc.Structure.Children)

	// Warn if start page has no start_button
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

	// Warn if blocks have points but enable_points is false
	if !l.doc.Settings.EnablePoints {
		l.warnBlocksWithPoints("start", l.doc.Start)
		l.warnBlocksWithPoints("finish", l.doc.Finish)
		l.warnChildrenBlockPoints("structure", l.doc.Structure.Children)
	}
}

func (l *linter) checkStructuralChildren(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		if child.Location != nil {
			loc := child.Location
			if len(loc.Navigation) == 0 {
				l.warnf(childPath+".location.navigation", "NO_NAVIGATION_BLOCKS",
					"location %q has no navigation blocks; players will see no clues to find it",
					loc.Name)
			}
			if len(loc.Content) == 0 {
				l.warnf(childPath+".location.content", "NO_CONTENT_BLOCKS",
					"location %q has no content blocks; players will see an empty page on check-in",
					loc.Name)
			}
		} else if child.Group != nil {
			if len(child.Group.Children) == 0 {
				l.warnf(childPath+".group", "EMPTY_GROUP",
					"group %q has no children", child.Group.Name)
			}
			l.checkGroupMinOneAutoAdvance(childPath+".group", *child.Group)
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
		if child.Location != nil {
			loc := *child.Location
			if loc.Points > 0 {
				l.warnf(childPath+".location.points", "POINTS_DISABLED",
					"location has points but enable_points is false")
			}
			l.warnBlocksWithPoints(childPath+".location.content", loc.Content)
			l.warnBlocksWithPoints(childPath+".location.navigation", loc.Navigation)
		} else if child.Group != nil {
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

// --- When / variable resolution checks ---

// collectAllDefinedVars walks the entire doc and records every variable name
// that any block can set (via the "sets" field). Called before semantic checks.
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
		if child.Location != nil {
			for _, b := range child.Location.Content {
				for _, v := range l.blockDocSetsVars(b) {
					vars[v] = true
				}
			}
			for _, b := range child.Location.Navigation {
				for _, v := range l.blockDocSetsVars(b) {
					vars[v] = true
				}
			}
		} else if child.Group != nil {
			l.collectVarsFromChildrenIntoSet(child.Group.Children, vars)
		}
	}
}

// checkWhenClausesInDoc checks that every Condition.Var in every when clause
// (on blocks, locations, and groups) refers to a variable that is actually set
// somewhere in the game.
func (l *linter) checkWhenClausesInDoc() {
	l.checkWhenInFixedContextBlocks("start", l.doc.Start)
	l.checkWhenInFixedContextBlocks("finish", l.doc.Finish)
	l.checkWhenInChildren("structure", l.doc.Structure.Children)
	l.checkUnusedVars()
}

// checkWhenInFixedContextBlocks warns when blocks in start/finish carry a when
// clause. The start and finish pages have no variable resolver and render all
// blocks unconditionally, so when clauses there are silently ignored.
func (l *linter) checkWhenInFixedContextBlocks(path string, blks []BlockDoc) {
	for i, b := range blks {
		blockPath := fmt.Sprintf("%s[%d]", path, i)
		wc, err := blockDocWhen(b)
		if err != nil {
			l.warnf(blockPath+".when", "WHEN_INVALID", "when clause is structurally invalid: %s", err)
			continue
		}
		if wc != nil {
			// Dead code: when clauses on start/finish are never evaluated.
			// Skip further checks to avoid spurious UNDEFINED_VAR warnings.
			l.warnf(blockPath+".when", "WHEN_ON_START_BLOCK",
				"when clauses on %s blocks are not evaluated; all blocks on this page are always shown", path)
			continue
		}
	}
}

func (l *linter) checkUnusedVars() {
	for varName := range l.definedVars {
		if !l.usedVars[varName] {
			l.warnf("", "UNUSED_VAR",
				"variable %q is set by a block but never referenced in any when clause", varName)
		}
	}
}

func (l *linter) checkWhenInChildren(path string, children []ChildDoc) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		if child.Location != nil {
			loc := child.Location
			locPath := childPath + ".location"
			l.checkWhenClause(locPath+".when", loc.When)
			l.checkWhenInBlocks(locPath+".content", loc.Content)
			l.checkWhenInBlocks(locPath+".navigation", loc.Navigation)
		} else if child.Group != nil {
			l.checkWhenClause(childPath+".group.when", child.Group.When)
			l.checkWhenInChildren(childPath+".group", child.Group.Children)
		}
	}
}

func (l *linter) checkWhenInBlocks(path string, blocks []BlockDoc) {
	for i, b := range blocks {
		blockPath := fmt.Sprintf("%s[%d]", path, i)
		wc, err := blockDocWhen(b)
		if err != nil {
			l.warnf(blockPath+".when", "WHEN_INVALID", "when clause is structurally invalid: %s", err)
			continue
		}
		l.checkWhenClause(blockPath+".when", wc)
	}
}

func (l *linter) checkWhenClause(path string, wc *WhenClause) {
	if wc == nil {
		return
	}
	if len(wc.AllOf) == 0 && len(wc.AnyOf) == 0 {
		l.warnf(path, "WHEN_VACUOUS",
			"when clause has no conditions; it is always true and has no effect — omit it or add conditions")
		return
	}
	for i, cond := range wc.AllOf {
		if cond.Var == "" {
			continue
		}
		l.usedVars[cond.Var] = true
		if !l.definedVars[cond.Var] && !isBuiltInVar(cond.Var) {
			l.warnf(fmt.Sprintf("%s.all_of[%d].var", path, i), "UNDEFINED_VAR",
				"condition references variable %q which is never set by any block in this game", cond.Var)
		}
	}
	for i, cond := range wc.AnyOf {
		if cond.Var == "" {
			continue
		}
		l.usedVars[cond.Var] = true
		if !l.definedVars[cond.Var] && !isBuiltInVar(cond.Var) {
			l.warnf(fmt.Sprintf("%s.any_of[%d].var", path, i), "UNDEFINED_VAR",
				"condition references variable %q which is never set by any block in this game", cond.Var)
		}
	}
}

// checkGroupMinOneAutoAdvance warns when a when clause inside a group references
// a variable that is only set within that same group, and the group has
// completion=minimum, minimum_required=1, and auto_advance=true (or nil/default).
// Because the team advances as soon as one location is completed, variables set
// by other locations in the group may never be written.
func (l *linter) checkGroupMinOneAutoAdvance(path string, g GroupDoc) {
	if g.Completion != CompletionMinimum || g.MinimumRequired != 1 {
		return
	}
	// nil defaults to true (matches import logic: g.AutoAdvance == nil || *g.AutoAdvance)
	if g.AutoAdvance != nil && !*g.AutoAdvance {
		return
	}

	groupVars := make(map[string]bool)
	l.collectVarsFromChildrenIntoSet(g.Children, groupVars)
	if len(groupVars) == 0 {
		return
	}

	l.checkGroupScopedWhenInChildren(path, g.Children, groupVars)
}

func (l *linter) checkGroupScopedWhenInChildren(path string, children []ChildDoc, groupVars map[string]bool) {
	for i, child := range children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		if child.Location != nil {
			loc := child.Location
			locPath := childPath + ".location"
			// Location-level when: check against full groupVars.
			// A location whose own blocks set x, but whose when also references x, is
			// a circular dependency (location hidden until x set, x only set inside it).
			l.checkGroupScopedWhen(locPath+".when", loc.When, groupVars)
			// Block-level when: exclude vars set by this location's own blocks.
			// "Self-reveal" — block B sets x, block C on the same location has when:{var:x} —
			// is valid; the sequence B→x→C plays out within a single location visit.
			crossVars := l.groupVarsExcludingSelf(groupVars, loc.Content, loc.Navigation)
			for j, b := range loc.Content {
				wc, _ := blockDocWhen(b) // errors already reported in checkWhenInBlocks
				l.checkGroupScopedWhen(fmt.Sprintf("%s.content[%d].when", locPath, j), wc, crossVars)
			}
			for j, b := range loc.Navigation {
				wc, _ := blockDocWhen(b) // errors already reported in checkWhenInBlocks
				l.checkGroupScopedWhen(fmt.Sprintf("%s.navigation[%d].when", locPath, j), wc, crossVars)
			}
		} else if child.Group != nil {
			l.checkGroupScopedWhenInChildren(childPath+".group", child.Group.Children, groupVars)
		}
	}
}

// groupVarsExcludingSelf returns a copy of groupVars with vars set by the given
// block slices removed. Used so that same-location setter+reference pairs do not
// trigger WHEN_UNREACHABLE_VAR (the self-reveal pattern is valid within one location visit).
func (l *linter) groupVarsExcludingSelf(groupVars map[string]bool, blockSlices ...[]BlockDoc) map[string]bool {
	var selfVars []string
	for _, slice := range blockSlices {
		for _, b := range slice {
			selfVars = append(selfVars, l.blockDocSetsVars(b)...)
		}
	}
	if len(selfVars) == 0 {
		return groupVars
	}
	out := make(map[string]bool, len(groupVars))
	for v := range groupVars {
		out[v] = true
	}
	for _, v := range selfVars {
		delete(out, v)
	}
	return out
}

func (l *linter) checkGroupScopedWhen(path string, wc *WhenClause, groupVars map[string]bool) {
	if wc == nil {
		return
	}
	for i, cond := range wc.AllOf {
		if groupVars[cond.Var] {
			l.warnf(fmt.Sprintf("%s.all_of[%d].var", path, i), "WHEN_UNREACHABLE_VAR",
				"condition references %q which is set within a min=1 auto-advance group; "+
					"the team may advance before this variable is ever written", cond.Var)
		}
	}
	for i, cond := range wc.AnyOf {
		if groupVars[cond.Var] {
			l.warnf(fmt.Sprintf("%s.any_of[%d].var", path, i), "WHEN_UNREACHABLE_VAR",
				"condition references %q which is set within a min=1 auto-advance group; "+
					"the team may advance before this variable is ever written", cond.Var)
		}
	}
}

// blockDocSetsVars returns all variable names that the given block doc defines.
// It reads from the standard top-level "sets" map and, via the registry,
// from block-type-specific sub-fields (e.g. options[*].sets on a choice block).
func (l *linter) blockDocSetsVars(b BlockDoc) []string {
	vars := collectSetsFromBlockDoc(b)
	if l.registry != nil {
		if t, ok := b["type"].(string); ok {
			vars = append(vars, l.registry.DocSetsVars(t, b)...)
		}
	}
	return vars
}

// checkSetsShape reports a "sets" value that is not an object of {name: value}.
// Catching it here gives the author a path; without it the failure surfaces
// later as an unmarshalling error with no location.
func (l *linter) checkSetsShape(path string, b BlockDoc) {
	raw, ok := b["sets"]
	if !ok {
		return
	}
	if _, ok := raw.(map[string]any); ok {
		return
	}
	l.errorf(path+".sets", "SETS_NOT_OBJECT",
		`"sets" must be an object {"name": "value"}`)
}

// collectSetsFromBlockDoc extracts variable names from the "sets" key of a block doc.
// "sets" is a map[string]any of {name: value}; any other shape is reported by
// checkSetsShape and contributes no vars.
func collectSetsFromBlockDoc(b BlockDoc) []string {
	raw, ok := b["sets"]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	var vars []string
	for name := range m {
		if name != "" {
			vars = append(vars, name)
		}
	}
	return vars
}

// blockDocWhen extracts the WhenClause from a BlockDoc by JSON roundtrip.
// Returns (nil, nil) when the "when" key is absent.
// Returns (nil, err) when the key is present but structurally invalid.
func blockDocWhen(b BlockDoc) (*WhenClause, error) {
	raw, ok := b["when"]
	if !ok {
		return nil, nil //nolint:nilnil // nil clause = absent, not an error
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshalling when clause: %w", err)
	}
	var wc WhenClause
	if err := json.Unmarshal(data, &wc); err != nil {
		return nil, fmt.Errorf("invalid when clause structure: %w", err)
	}
	return &wc, nil
}

// isBuiltInVar reports whether name is a built-in variable provided by the
// runtime (not set by any block in the game doc). Referencing a built-in in a
// when clause is valid even though it never appears in definedVars.
//
// Built-ins (from specgen.BuiltInVarSpecs):
//
//	player.points (legacy spelling: points), run.started_at, game.team_count
//	location.<slug>.visited, location.<slug>.checked_in
//	group.<name>.completed
func isBuiltInVar(name string) bool {
	switch name {
	case "player.points", "points", "run.started_at", "game.team_count":
		return true
	}
	if after, ok := strings.CutPrefix(name, "location."); ok {
		dot := strings.LastIndex(after, ".")
		if dot >= 0 {
			attr := after[dot+1:]
			return attr == "visited" || attr == "checked_in"
		}
	}
	if after, ok := strings.CutPrefix(name, "group."); ok {
		return strings.HasSuffix(after, ".completed")
	}
	return false
}
