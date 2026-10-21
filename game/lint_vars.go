package game

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// checkBlockVarsDoc checks the shape of a block's sets and depends. Only an
// interactive block can set a variable, because completing it is the player
// doing something; only a content block can depend on one, because a block
// that waits on another block is a chain nothing on the screen explains.
func (l *linter) checkBlockVarsDoc(path, typ string, b BlockDoc) {
	interactive := l.registry != nil && l.registry.IsInteractive(typ)
	chooses := false
	if l.registry != nil {
		_, chooses = l.registry.DocVars(typ, b)
	}

	if raw, ok := b["sets"]; ok && raw != nil {
		s, isString := raw.(string)
		switch {
		case !isString:
			l.errorf(path+".sets", "SETS_NOT_STRING",
				"sets must be a single variable name: completing a block is one event, so it sets one variable")
		case s == "":
		case l.registry != nil && !interactive:
			l.errorf(path+".sets", "SETS_ON_STATIC_BLOCK",
				"block type %q asks nothing of the player, so there is no moment for it to set %q; "+
					"move it to an interactive block", typ, s)
		case chooses:
			l.errorf(path+".sets", "SETS_ON_CHOICE",
				"block type %q sets the variable of whichever option the player picks, so its own sets "+
					"is never used; give each option its variable instead", typ)
		case l.registry != nil && !l.registry.CanSetVars(typ):
			l.errorf(path+".sets", "SETS_UNSUPPORTED",
				"block type %q is completed outside the usual answer, so there is no moment for it to "+
					"set %q; move it to another interactive block", typ, s)
		case !ValidVarName(s):
			l.errorf(path+".sets", "VAR_NAME_INVALID", invalidVarNameMessage, s)
		}
	}

	raw, ok := b["depends"]
	if !ok || raw == nil {
		return
	}
	names, isList := docStrings(raw)
	if !isList {
		l.errorf(path+".depends", "DEPENDS_NOT_LIST", "depends must be a list of variable names")
		return
	}
	if interactive && len(names) > 0 {
		l.errorf(path+".depends", "DEPENDS_ON_INTERACTIVE_BLOCK",
			"block type %q is something the player answers, and answers cannot wait on one another; "+
				"gate the objective it sits in, or a content block beside it", typ)
		return
	}
	l.checkDependsNames(path+".depends", names)
}

const invalidVarNameMessage = "variable %q must be lowercase letters, digits and underscores, " +
	"not starting with a digit"

func (l *linter) checkDependsNames(path string, names []string) {
	seen := make(map[string]bool, len(names))
	for i, name := range names {
		p := fmt.Sprintf("%s[%d]", path, i)
		switch {
		case name == "":
			l.errorf(p, "DEPENDS_EMPTY_NAME", "a depends entry names no variable, so its gate can never open")
		case !ValidVarName(name):
			l.errorf(p, "VAR_NAME_INVALID", invalidVarNameMessage, name)
		case seen[name]:
			l.warnf(p, "DEPENDS_DUPLICATE", "%q is already listed", name)
		}
		seen[name] = true
	}
}

// docStrings reads a list of strings from a decoded document, which holds
// []any, or from one built in Go.
func docStrings(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return v, true
	case DependsField:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

// varBlock is one block as the variable checks see it.
type varBlock struct {
	path        string
	ctx         BlockContext
	node        *varNode // nil on the start and finish pages
	interactive bool
	sets        []string
	// chooses means the player picks which of sets to write.
	chooses bool
	// exclusive means the player picks one of sets, never several.
	exclusive bool
	depends   []string
	draft     bool
}

// varNode is one objective as the variable checks see it.
type varNode struct {
	path     string
	doc      *ObjectiveDoc
	parent   *varNode
	children []*varNode // published only: a draft is never in play
	draft    bool       // drafted itself or beneath a draft
	blocks   []*varBlock
}

func (n *varNode) isWithin(ancestor *varNode) bool {
	for p := n; p != nil; p = p.parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// varGraph indexes who sets and who reads each variable.
type varGraph struct {
	root    *varNode
	nodes   []*varNode // every node, drafts included, in document order
	pages   []*varBlock
	setters map[string][]*varBlock
	used    map[string]bool
}

func (l *linter) buildVarGraph() *varGraph {
	g := &varGraph{setters: make(map[string][]*varBlock), used: make(map[string]bool)}
	for i, b := range l.doc.Start {
		g.pages = append(g.pages, l.varBlock(fmt.Sprintf("start[%d]", i), ContextStart, nil, b))
	}
	for i, b := range l.doc.Finish {
		g.pages = append(g.pages, l.varBlock(fmt.Sprintf("finish[%d]", i), ContextFinish, nil, b))
	}
	g.root = l.varNode(g, "structure", &l.doc.Structure, nil, false)
	for _, b := range g.pages {
		g.index(b)
	}
	for _, n := range g.nodes {
		for _, name := range n.doc.Depends {
			g.used[name] = true
		}
		for _, b := range n.blocks {
			g.index(b)
		}
	}
	return g
}

// eachGate calls fn with every depends list in the quest: on the pages, on
// objectives and on their blocks, drafts included. inDraft says whether the
// gate is out of play.
func (g *varGraph) eachGate(fn func(path string, names []string, inDraft bool)) {
	for _, b := range g.pages {
		fn(b.path+".depends", b.depends, false)
	}
	for _, n := range g.nodes {
		fn(n.path+".depends", n.doc.Depends, n.draft)
		for _, b := range n.blocks {
			fn(b.path+".depends", b.depends, b.draft)
		}
	}
}

func (g *varGraph) index(b *varBlock) {
	for _, name := range b.sets {
		g.setters[name] = append(g.setters[name], b)
	}
	for _, name := range b.depends {
		g.used[name] = true
	}
}

func (l *linter) varNode(g *varGraph, path string, doc *ObjectiveDoc, parent *varNode, draft bool) *varNode {
	n := &varNode{path: path, doc: doc, parent: parent, draft: draft || doc.IsDraft()}
	g.nodes = append(g.nodes, n)
	for i, b := range doc.Proof.Blocks {
		n.blocks = append(
			n.blocks,
			l.varBlock(fmt.Sprintf("%s.proof.blocks[%d]", path, i), ContextObjectiveProof, n, b),
		)
	}
	for i, b := range doc.Reveal.Blocks {
		n.blocks = append(
			n.blocks,
			l.varBlock(fmt.Sprintf("%s.reveal.blocks[%d]", path, i), ContextObjectiveReveal, n, b),
		)
	}
	for i := range doc.Children {
		child := l.varNode(g, fmt.Sprintf("%s.children[%d]", path, i), &doc.Children[i], n, n.draft)
		if !doc.Children[i].IsDraft() {
			n.children = append(n.children, child)
		}
	}
	return n
}

func (l *linter) varBlock(path string, ctx BlockContext, node *varNode, doc BlockDoc) *varBlock {
	b := &varBlock{path: path, ctx: ctx, node: node, draft: node != nil && node.draft}
	typ, _ := doc["type"].(string)
	b.interactive = l.registry.IsInteractive(typ)
	if dv, ok := l.registry.DocVars(typ, doc); ok {
		b.sets = dv.Names
		b.chooses = true
		b.exclusive = dv.Exclusive
	} else if s, _ := doc["sets"].(string); s != "" && l.registry.CanSetVars(typ) {
		b.sets = []string{s}
	}
	if !b.interactive {
		b.depends, _ = docStrings(doc["depends"])
	}
	return b
}

// checkVariables runs the checks that need the whole quest: whether every gate
// has something that opens it, and whether a player can reach that something
// before they need it. It runs only on a document whose shape is sound, so it
// trusts the names it reads.
func (l *linter) checkVariables() {
	if l.registry == nil {
		return
	}
	g := l.buildVarGraph()
	dead := l.checkVarReferences(g)
	l.checkUnusedVars(g)
	l.checkExclusiveDepends(g)
	open, avail := g.reachable()
	l.checkUnsatisfiable(g, open, avail, dead)
	l.checkDependsTooLate(g)
	l.checkRandomisedWindows(g)
	l.checkExclusiveBands(g)
	l.checkBandClosesFirst(g)
}

// checkVarReferences reports gates on names nothing in play sets, and returns
// those names so later checks do not blame the same gate twice.
func (l *linter) checkVarReferences(g *varGraph) map[string]bool {
	dead := make(map[string]bool)
	check := func(path string, names []string, inDraft bool) {
		for i, name := range names {
			p := fmt.Sprintf("%s[%d]", path, i)
			setters := g.setters[name]
			if len(setters) == 0 {
				dead[name] = true
				l.errorf(p, "UNDEFINED_VAR",
					"no block sets %q, so this gate can never open; set it from an interactive block "+
						"or a choice option", name)
				continue
			}
			if !slices.ContainsFunc(setters, func(b *varBlock) bool { return !b.draft }) {
				dead[name] = true
				// A draft waiting on a draft is consistent: both go into play
				// together when the author publishes.
				if !inDraft {
					l.warnf(p, "VAR_ONLY_SET_IN_DRAFT",
						"%q is only set inside a draft, so this gate stays shut until that draft is published",
						name)
				}
			}
		}
	}
	g.eachGate(check)
	return dead
}

// checkUnusedVars warns about a block's sets that nothing reads. A choice is
// left alone: its variables are also how each option is told apart, so they
// exist whether or not anything waits on them.
func (l *linter) checkUnusedVars(g *varGraph) {
	each := func(b *varBlock) {
		if b.chooses || len(b.sets) != 1 || g.used[b.sets[0]] {
			return
		}
		l.warnf(b.path+".sets", "UNUSED_VAR",
			"nothing depends on %q, so setting it changes nothing", b.sets[0])
	}
	for _, b := range g.pages {
		each(b)
	}
	for _, n := range g.nodes {
		for _, b := range n.blocks {
			each(b)
		}
	}
}

// exclusiveGroup names the single-select block that is the only setter of
// name, or "" when the name can be set some other way too.
func (g *varGraph) exclusiveGroup(name string) string {
	setters := g.setters[name]
	if len(setters) == 1 && setters[0].exclusive {
		return setters[0].path
	}
	return ""
}

// checkExclusiveDepends rejects one gate that needs two options of the same
// single-select choice, since the player can only ever pick one.
func (l *linter) checkExclusiveDepends(g *varGraph) {
	g.eachGate(func(path string, names []string, _ bool) {
		byGroup := make(map[string]string)
		for i, name := range names {
			group := g.exclusiveGroup(name)
			if group == "" {
				continue
			}
			if other, ok := byGroup[group]; ok && other != name {
				l.errorf(fmt.Sprintf("%s[%d]", path, i), "DEPENDS_EXCLUSIVE_CHOICES",
					"%q and %q are options of the same choice at %s, and a player picks only one, "+
						"so this gate can never open", other, name, group)
				continue
			}
			byGroup[group] = name
		}
	})
}

// reachable plays the quest optimistically: every interactive block a player
// can reach is assumed solved, and every choice is assumed to take every
// option. What stays shut even then can never open. Bands and windows are
// ignored except where an ordered section makes one child wait on the last.
func (g *varGraph) reachable() (map[*varNode]bool, map[string]bool) {
	r := &reachability{
		avail:     make(map[string]bool),
		open:      map[*varNode]bool{g.root: true},
		harvested: make(map[*varNode]bool),
	}
	for _, b := range g.pages {
		if b.ctx == ContextStart {
			r.collect(b)
		}
	}
	for changed := true; changed; {
		changed = r.visit(g.root)
	}
	return r.open, r.avail
}

// reachability is one optimistic play-through, grown until nothing changes.
type reachability struct {
	avail     map[string]bool
	open      map[*varNode]bool
	harvested map[*varNode]bool
}

func (r *reachability) collect(b *varBlock) {
	if !b.interactive {
		return
	}
	for _, name := range b.sets {
		r.avail[name] = true
	}
}

// visit opens what it can beneath an open node and reports whether anything
// changed.
func (r *reachability) visit(n *varNode) bool {
	changed := false
	if !r.harvested[n] {
		r.harvested[n] = true
		changed = true
		for _, b := range n.blocks {
			r.collect(b)
		}
	}
	for i, c := range n.children {
		if !r.open[c] && r.canOpen(n, i) {
			r.open[c] = true
			changed = true
		}
		if r.open[c] && r.visit(c) {
			changed = true
		}
	}
	return changed
}

// canOpen reports whether a parent's i-th child can open: its variables are
// all available and, in an ordered section, every child before it can be
// completed.
func (r *reachability) canOpen(parent *varNode, i int) bool {
	if !DependsMet(parent.children[i].doc.Depends, r.avail) {
		return false
	}
	if parent.doc.Routing != RouteStrategyOrdered {
		return true
	}
	for _, earlier := range parent.children[:i] {
		if !r.completable(earlier) {
			return false
		}
	}
	return true
}

func (r *reachability) completable(n *varNode) bool {
	if !r.open[n] {
		return false
	}
	if len(n.children) == 0 {
		return true
	}
	done := 0
	for _, c := range n.children {
		if r.completable(c) {
			done++
		}
	}
	return done >= FillBand(n.doc.ChildrenMin, n.doc.ChildrenMax, len(n.children)).Min
}

// unmet returns the names in deps the play-through never made available, or
// nil when one of them is dead: that gate has already been reported.
func unmet(deps []string, avail, dead map[string]bool) []string {
	var out []string
	for _, name := range deps {
		if dead[name] {
			return nil
		}
		if !avail[name] {
			out = append(out, name)
		}
	}
	return out
}

func (l *linter) reportUnsatisfiable(path, what string, names []string) {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = fmt.Sprintf("%q", name)
	}
	l.errorf(path, "DEPENDS_UNSATISFIABLE",
		"%s waits on %s, but every block that sets it is behind this gate or behind another gate "+
			"that waits on it, so no player can reach it", what, strings.Join(quoted, ", "))
}

// checkUnsatisfiable reports gates that every name has a setter for, but that
// still never open: the setter sits behind the gate itself, or behind another
// gate waiting on this one. Only the highest shut objective is reported, since
// everything beneath it is shut for the same reason.
func (l *linter) checkUnsatisfiable(g *varGraph, open map[*varNode]bool, avail, dead map[string]bool) {
	for _, n := range g.nodes {
		if n.draft || n.parent == nil || !open[n.parent] || open[n] {
			continue
		}
		if m := unmet(n.doc.Depends, avail, dead); len(m) > 0 {
			l.reportUnsatisfiable(n.path+".depends", fmt.Sprintf("objective %q", n.doc.Slug), m)
		}
	}
	var blocksInPlay []*varBlock
	for _, n := range g.nodes {
		if !n.draft && open[n] {
			blocksInPlay = append(blocksInPlay, n.blocks...)
		}
	}
	for _, b := range blocksInPlay {
		if m := unmet(b.depends, avail, dead); len(m) > 0 {
			l.reportUnsatisfiable(b.path+".depends", "this block", m)
		}
	}
	l.checkPagesUnsatisfiable(g, avail, dead)
}

// checkPagesUnsatisfiable is checkUnsatisfiable for the start and finish pages.
// The finish page comes after everything else, so what it sets opens only what
// is on it: a block there can wait on an answer beside it.
func (l *linter) checkPagesUnsatisfiable(g *varGraph, avail, dead map[string]bool) {
	atFinish := maps.Clone(avail)
	for _, b := range g.pages {
		if b.ctx == ContextFinish && b.interactive {
			for _, name := range b.sets {
				atFinish[name] = true
			}
		}
	}
	for _, b := range g.pages {
		pageAvail := avail
		if b.ctx == ContextFinish {
			pageAvail = atFinish
		}
		if m := unmet(b.depends, pageAvail, dead); len(m) > 0 {
			l.reportUnsatisfiable(b.path+".depends", "this block", m)
		}
	}
}

// checkDependsTooLate warns about a proof block waiting on something only set
// once the proof has cleared. The proof zone is replaced by the reveal at that
// moment, so the block would appear on a page the player has already left.
func (l *linter) checkDependsTooLate(g *varGraph) {
	for _, n := range g.nodes {
		if n.draft {
			continue
		}
		for _, b := range n.blocks {
			if b.ctx != ContextObjectiveProof {
				continue
			}
			for i, name := range b.depends {
				if setOnlyAfterProof(n, livingSetters(g.setters[name])) {
					l.warnf(fmt.Sprintf("%s.depends[%d]", b.path, i), "BLOCK_DEPENDS_TOO_LATE",
						"%q is only set after this objective's proof clears, when its proof blocks are no "+
							"longer shown; move this block to the reveal", name)
				}
			}
		}
	}
}

// setOnlyAfterProof reports whether every setter is in n's reveal or beneath
// n, both of which a player reaches only once n's proof has cleared.
func setOnlyAfterProof(n *varNode, setters []*varBlock) bool {
	if len(setters) == 0 {
		return false
	}
	return !slices.ContainsFunc(setters, func(s *varBlock) bool {
		if s.node == nil || !s.node.isWithin(n) {
			return true
		}
		return s.node == n && s.ctx == ContextObjectiveProof
	})
}

func livingSetters(setters []*varBlock) []*varBlock {
	var out []*varBlock
	for _, s := range setters {
		if !s.draft {
			out = append(out, s)
		}
	}
	return out
}

// checkRandomisedWindows warns about a gated child in a randomised section
// that offers only a few children at a time. A shut child still takes one of
// those places, so the player is shown something they cannot open in place of
// something they could.
func (l *linter) checkRandomisedWindows(g *varGraph) {
	for _, n := range g.nodes {
		if n.draft || n.doc.Routing != RouteStrategyRandomised {
			continue
		}
		if n.doc.MaxNext <= 0 || n.doc.MaxNext >= len(n.children) {
			continue
		}
		for _, c := range n.children {
			if len(c.doc.Depends) > 0 {
				l.warnf(c.path+".depends", "DEPENDS_IN_RANDOMISED_WINDOW",
					"objective %q is gated inside a section that offers %d at a time, so while it is "+
						"shut it can fill a place a player could have used", c.doc.Slug, n.doc.MaxNext)
			}
		}
	}
}

// checkExclusiveBands warns about a section that needs more children than a
// player can open, because some are gated on different options of the same
// single-select choice and only one option's children will ever open.
func (l *linter) checkExclusiveBands(g *varGraph) {
	for _, n := range g.nodes {
		if n.draft || len(n.children) == 0 {
			continue
		}
		lost, groups := g.exclusiveLoss(n)
		need := FillBand(n.doc.ChildrenMin, n.doc.ChildrenMax, len(n.children)).Min
		if lost == 0 || need <= len(n.children)-lost {
			continue
		}
		l.warnf(n.path, "BAND_NEEDS_EXCLUSIVE_CHOICES",
			"objective %q needs %d of its %d children, but some are gated on different options of the "+
				"same choice (%s) and a player can open at most %d; lower children_min or move the "+
				"branches into a section of their own", n.doc.Slug, need, len(n.children),
			strings.Join(groups, ", "), len(n.children)-lost)
	}
}

// exclusiveLoss counts the children of n that can never open alongside the
// rest: for each single-select choice its children are gated on, all but the
// best-served option's children. It also names those choices.
func (g *varGraph) exclusiveLoss(n *varNode) (int, []string) {
	// For each choice, how many children each of its options opens.
	perGroup := make(map[string]map[string]int)
	for _, c := range n.children {
		counted := make(map[string]bool)
		for _, name := range c.doc.Depends {
			group := g.exclusiveGroup(name)
			if group == "" || counted[group] {
				continue
			}
			counted[group] = true
			if perGroup[group] == nil {
				perGroup[group] = make(map[string]int)
			}
			perGroup[group][name]++
		}
	}
	lost := 0
	var groups []string
	for group, options := range perGroup {
		if len(options) < 2 { //nolint:mnd // one option opens freely; it takes two to exclude
			continue
		}
		total, best := 0, 0
		for _, count := range options {
			total += count
			best = max(best, count)
		}
		lost += total - best
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return lost, groups
}

// checkBandClosesFirst warns about a gated child whose keys are all held by its
// siblings, in a section that closes once that many siblings are done. The
// section closes on the last of them, and a child of a closed section stays
// locked. It warns rather than errs: a key in a sibling's proof can be set
// before that sibling finishes, if something else in the proof is left undone.
func (l *linter) checkBandClosesFirst(g *varGraph) {
	for _, n := range g.nodes {
		if n.draft || len(n.children) == 0 {
			continue
		}
		closesAt := FillBand(n.doc.ChildrenMin, n.doc.ChildrenMax, len(n.children)).Max
		for _, c := range n.children {
			needed := g.siblingsHolding(n, c)
			if len(needed) == 0 || len(needed) < closesAt {
				continue
			}
			l.warnf(c.path+".depends", "BAND_CLOSES_FIRST",
				"objective %q opens only once %d of its siblings are done, but %q closes after %d, so it "+
					"is likely to stay locked; raise children_max or set its variables somewhere else",
				c.doc.Slug, len(needed), n.doc.Slug, closesAt)
		}
	}
}

// siblingsHolding returns the siblings of c, children of parent, that alone set
// one of the variables c waits on: every published setter of that variable is
// inside that one sibling.
func (g *varGraph) siblingsHolding(parent, c *varNode) map[*varNode]bool {
	needed := make(map[*varNode]bool)
	for _, name := range c.doc.Depends {
		var holder *varNode
		for _, setter := range livingSetters(g.setters[name]) {
			sibling := childOnPathTo(parent, setter.node)
			if sibling == nil || sibling == c || (holder != nil && holder != sibling) {
				holder = nil
				break
			}
			holder = sibling
		}
		if holder != nil {
			needed[holder] = true
		}
	}
	return needed
}

// childOnPathTo returns the child of parent that n sits within, or nil when n
// is not beneath parent.
func childOnPathTo(parent, n *varNode) *varNode {
	for ; n != nil; n = n.parent {
		if n.parent == parent {
			return n
		}
	}
	return nil
}
