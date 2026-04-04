package game

import (
	"encoding/json"
	"fmt"
)

// LintResult holds errors and warnings from linting a GameDoc.
type LintResult struct {
	Errors   []LintDiag // Must fix before importing
	Warnings []LintDiag // Should fix but won't block import
}

// LintDiag is a single diagnostic message with a path and code.
type LintDiag struct {
	Path    string // e.g. "structure.children[0].location.content[1]"
	Code    string // e.g. "SLUG_DUPLICATE", "INVALID_CONTEXT"
	Message string
}

// IsValid returns true if there are no errors (warnings are acceptable).
func (r LintResult) IsValid() bool {
	return len(r.Errors) == 0
}

// Lint validates a GameDoc in three layers: schema, semantic, structural.
// registry is used to check valid block types and contexts; pass blocks.Registry().
func Lint(doc *GameDoc, registry BlockRegistry) LintResult {
	l := &linter{doc: doc, registry: registry}
	l.run()
	return l.result
}

type linter struct {
	doc      *GameDoc
	registry BlockRegistry
	result   LintResult
	slugs    map[string]bool
	blockIDs map[string]bool
}

func (l *linter) run() {
	l.slugs = make(map[string]bool)
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
	l.checkNavigation(path+".navigation", s.Navigation)
	l.checkCompletion(path, s.Completion, s.MinimumRequired)
	for i, child := range s.Children {
		l.checkChildDoc(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
}

func (l *linter) checkGroupDoc(path string, g GroupDoc) {
	if g.Name == "" {
		l.errorf(path+".name", "MISSING_GROUP_NAME", "group name is required")
	}
	l.checkRouting(path+".routing", g.Routing)
	l.checkNavigation(path+".navigation", g.Navigation)
	l.checkCompletion(path, g.Completion, g.MinimumRequired)
	for i, child := range g.Children {
		l.checkChildDoc(fmt.Sprintf("%s.children[%d]", path, i), child)
	}
}

func (l *linter) checkChildDoc(path string, child ChildDoc) {
	if child.Location != nil {
		l.checkLocationDoc(path+".location", *child.Location)
	} else if child.Group != nil {
		l.checkGroupDoc(path+".group", *child.Group)
	} else {
		l.errorf(path, "EMPTY_CHILD", "child has neither location nor group")
	}
}

func (l *linter) checkLocationDoc(path string, loc LocationDoc) {
	if loc.Slug == "" {
		l.errorf(path+".slug", "MISSING_SLUG", "location slug is required")
	}
	if loc.Name == "" {
		l.errorf(path+".name", "MISSING_LOCATION_NAME", "location name is required")
	}
	if loc.Points < 0 {
		l.errorf(path+".points", "INVALID_POINTS", "points must be non-negative")
	}
	if loc.Marker != nil {
		if loc.Marker.Lat == 0 && loc.Marker.Lng == 0 {
			l.warnf(path+".marker", "ZERO_COORDINATES", "marker has zero coordinates; omit marker if location has no map pin")
		}
	}
	for i, b := range loc.Content {
		l.checkBlockDoc(fmt.Sprintf("%s.content[%d]", path, i), b, ContextLocationContent)
	}
	for i, b := range loc.Clues {
		l.checkBlockDoc(fmt.Sprintf("%s.clues[%d]", path, i), b, ContextLocationClues)
	}
	for i, b := range loc.Tasks {
		l.checkBlockDoc(fmt.Sprintf("%s.tasks[%d]", path, i), b, ContextTask)
	}
	for i, b := range loc.Checkpoint {
		l.checkBlockDoc(fmt.Sprintf("%s.checkpoint[%d]", path, i), b, ContextCheckpoint)
	}
}

func (l *linter) checkBlockDoc(path string, b BlockDoc, ctx BlockContext) {
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
			knownSet := make(map[string]bool, len(known)+3)
			for _, f := range known {
				knownSet[f] = true
			}
			// Promoted fields are always valid
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

func (l *linter) checkNavigation(path string, n NavigationMode) {
	switch n {
	case NavigationMap, NavigationLabelledMap, NavigationList, NavigationCustom, NavigationTasks:
		// valid
	default:
		l.errorf(path, "INVALID_NAVIGATION", "invalid navigation value %q", n)
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
	l.checkBlockContexts(path+".clues", loc.Clues, ContextLocationClues)
	l.checkBlockContexts(path+".tasks", loc.Tasks, ContextTask)
	l.checkBlockContexts(path+".checkpoint", loc.Checkpoint, ContextCheckpoint)
	l.trackBlockIDs(path+".content", loc.Content)
	l.trackBlockIDs(path+".clues", loc.Clues)
	l.trackBlockIDs(path+".tasks", loc.Tasks)
	l.trackBlockIDs(path+".checkpoint", loc.Checkpoint)
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
		if child.Location != nil {
			loc := *child.Location
			if loc.Points > 0 {
				l.warnf(childPath+".location.points", "POINTS_DISABLED",
					"location has points but enable_points is false")
			}
			l.warnBlocksWithPoints(childPath+".location.content", loc.Content)
			l.warnBlocksWithPoints(childPath+".location.clues", loc.Clues)
			l.warnBlocksWithPoints(childPath+".location.tasks", loc.Tasks)
			l.warnBlocksWithPoints(childPath+".location.checkpoint", loc.Checkpoint)
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
