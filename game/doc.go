package game

// GameDoc is the top-level v8 JSON game document.
// The DB is the source of truth; this document is a lossless projection.
// Export reads from DB; import writes to it. Round-trips are lossless.
type GameDoc struct {
	Rapua     string       `json:"rapua"`
	ID        string       `json:"id,omitempty"`
	Name      string       `json:"name"`
	Settings  SettingsDoc  `json:"settings"`
	Start     []BlockDoc   `json:"start"`
	Finish    []BlockDoc   `json:"finish"`
	Structure StructureDoc `json:"structure"`
}

// SettingsDoc mirrors QuestSettings fields.
type SettingsDoc struct {
	MustCheckOut    bool `json:"must_check_out"`
	ShowTeamCount   bool `json:"show_team_count"`
	EnablePoints    bool `json:"enable_points"`
	ShowLeaderboard bool `json:"show_leaderboard"`
}

type StructureDoc struct {
	Routing         RouteStrategy  `json:"routing"`
	Completion      CompletionType `json:"completion"`
	MinimumRequired int            `json:"minimum_required,omitempty"`
	Children        []ChildDoc     `json:"children"`
}

// GroupDoc: AutoAdvance defaults to true when omitted. Set it to false to keep
// players in the group after its completion criteria are met.
type GroupDoc struct {
	ID              string         `json:"id,omitempty"`
	Name            string         `json:"name"`
	Color           string         `json:"color"`
	Routing         RouteStrategy  `json:"routing"`
	Completion      CompletionType `json:"completion"`
	MinimumRequired int            `json:"minimum_required,omitempty"`
	AutoAdvance     *bool          `json:"auto_advance,omitempty"`
	When            *WhenClause    `json:"when,omitempty"`
	Children        []ChildDoc     `json:"children"`
}

type LocationDoc struct {
	ID         string      `json:"id,omitempty"`
	Slug       string      `json:"slug"`
	Name       string      `json:"name"`
	Points     int         `json:"points,omitempty"`
	When       *WhenClause `json:"when,omitempty"`
	Marker     *MarkerDoc  `json:"marker,omitempty"`
	Content    []BlockDoc  `json:"content"`
	Navigation []BlockDoc  `json:"navigation,omitempty"`
}

type MarkerDoc struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// ObjectiveDoc renders one fixed design rather than the freeform block canvas a
// Location uses: exactly two contexts, proof then reveal, no pre-proof context.
type ObjectiveDoc struct {
	ID     string              `json:"id,omitempty"`
	Slug   string              `json:"slug"`
	Title  string              `json:"title"`
	When   *WhenClause         `json:"when,omitempty"`
	Proof  ObjectiveContextDoc `json:"proof"`
	Reveal ObjectiveContextDoc `json:"reveal"`
}

// ObjectiveContextDoc is one of an objective's two contexts (proof or reveal).
// Its Sets fire once, the moment every block in the context completes.
type ObjectiveContextDoc struct {
	Blocks []BlockDoc `json:"blocks,omitempty"`
	Sets   SetsField  `json:"sets,omitempty"`
}

// ChildDoc is a tagged union: exactly one of Location, Group, or Objective is set.
// Custom marshal/unmarshal is in doc_marshal.go.
type ChildDoc struct {
	Location  *LocationDoc
	Group     *GroupDoc
	Objective *ObjectiveDoc
}

// BlockDoc is a flat map with a "type" discriminator plus all block-specific fields.
// "id" and "points" are promoted to the top level.
// Export includes "id"; create-import omits it so new UUIDs are generated.
type BlockDoc map[string]any
