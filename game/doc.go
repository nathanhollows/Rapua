package game

// GameDoc is the top-level v8 JSON game document.
// The DB is the source of truth; this document is a lossless projection.
// Export reads from DB; import writes to it. Round-trips are lossless.
type GameDoc struct {
	Rapua    string      `json:"rapua"`
	ID       string      `json:"id,omitempty"`
	Name     string      `json:"name"`
	Settings SettingsDoc `json:"settings"`
	Start    []BlockDoc  `json:"start"`
	Finish   []BlockDoc  `json:"finish"`
	// Structure is the root objective. It is an ordinary node with no parent,
	// not a separate container type: one recursive concept all the way down.
	Structure ObjectiveDoc `json:"structure"`
}

// SettingsDoc mirrors QuestSettings fields.
type SettingsDoc struct {
	EnablePoints    bool `json:"enable_points"`
	ShowLeaderboard bool `json:"show_leaderboard"`
}

// ObjectiveDoc is the single recursive node of a quest. A leaf is an objective
// in the everyday sense: two block contexts, proof then reveal. A node with
// children is a chapter, routing players through them and completing on its own
// band. Nothing distinguishes the two but the presence of children, so a
// chapter may carry real proof content of its own, and a leaf may later grow
// children without changing type.
type ObjectiveDoc struct {
	ID    string `json:"id,omitempty"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	// Draft holds an objective out of play without moving it, gating it and
	// everything beneath it. A drafted node keeps its place in the tree and its
	// children, so publishing puts back exactly what was there.
	//
	// A pointer because omitting the key and writing false are different
	// statements. A document that never mentions draft (one exported before the
	// field existed, or written by other tooling) leaves the stored state
	// alone; only "draft": false publishes something.
	Draft   *bool               `json:"draft,omitempty"`
	Color   string              `json:"color,omitempty"`
	Depends DependsField        `json:"depends,omitempty"`
	Proof   ObjectiveContextDoc `json:"proof"`
	Reveal  ObjectiveContextDoc `json:"reveal"`

	// Routing orders the children below. Meaningless without them.
	Routing RouteStrategy `json:"routing,omitempty"`
	// ChildrenMin and ChildrenMax are the completion band: min is when the
	// player may finish this node, max is when it finishes on its own. Both
	// are pointers because an explicit 0 is not the same as an omitted bound.
	// See FillBand for how the pair resolves.
	ChildrenMin *int `json:"children_min,omitempty"`
	ChildrenMax *int `json:"children_max,omitempty"`
	// MaxNext caps how many children a randomised node offers at once.
	MaxNext int `json:"max_next,omitempty"`
	// FinishLabel names the finish button. Only a node in a range ever shows
	// one, so lint warns when it is set on a node that auto-completes.
	FinishLabel string         `json:"finish_label,omitempty"`
	Children    []ObjectiveDoc `json:"children,omitempty"`
}

// ObjectiveContextDoc is one of an objective's two contexts (proof or reveal).
// Its Sets fire once, the moment every block in the context completes.
type ObjectiveContextDoc struct {
	Blocks []BlockDoc `json:"blocks,omitempty"`
	Sets   SetsField  `json:"sets,omitempty"`
}

// Band is the resolved completion range over a node's children.
//
// Min is the count at which the player may finish the node; Max is the count
// at which it finishes by itself. When they are equal there is nothing for the
// player to decide and the node auto-completes; when Min is lower, reaching Min
// only offers a finish button and the press is what completes the node.
type Band struct {
	Min int
	Max int
}

// FillBand resolves the optional children_min/children_max pair against the
// number of children.
//
// Omitting both means every child is required: [n, n], which auto-completes.
// Naming either bound switches the node into explicit-band mode, where the
// omitted end of the range falls back to its widest value: 0 for min, the
// child count for max. That is what separates an omitted min from an explicit
// 0, which is a genuinely different node ([n, n] versus [0, n]).
func FillBand(minChildren, maxChildren *int, childCount int) Band {
	if minChildren == nil && maxChildren == nil {
		return Band{Min: childCount, Max: childCount}
	}
	band := Band{Min: 0, Max: childCount}
	if minChildren != nil {
		band.Min = *minChildren
	}
	if maxChildren != nil {
		band.Max = *maxChildren
	}
	return band
}

// AutoCompletes reports whether the node finishes without the player choosing
// to, which is exactly the case where the band has no range to decide within.
func (b Band) AutoCompletes() bool { return b.Min >= b.Max }

// Band resolves this node's completion band against its published children.
//
// Drafts are excluded because the player engine never loads them: a band
// counting a child no run can reach is a band no run can meet. This is the same
// count the frontier works from, which loads published rows only.
func (o ObjectiveDoc) Band() Band {
	return FillBand(o.ChildrenMin, o.ChildrenMax, o.PublishedChildCount())
}

// IsDraft reports whether the node is held out of play. An omitted key is not a
// draft: it is a document with no opinion, which reads as published for a node
// that has no stored state to keep.
func (o ObjectiveDoc) IsDraft() bool { return o.Draft != nil && *o.Draft }

// PublishedChildCount is how many of a node's children are in play.
func (o ObjectiveDoc) PublishedChildCount() int {
	count := 0
	for _, child := range o.Children {
		if !child.IsDraft() {
			count++
		}
	}
	return count
}

// BlockDoc is a flat map with a "type" discriminator plus all block-specific fields.
// "id" and "points" are promoted to the top level.
// Export includes "id"; create-import omits it so new UUIDs are generated.
type BlockDoc map[string]any
