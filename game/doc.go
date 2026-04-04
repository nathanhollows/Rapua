package game

// GameDoc is the top-level v7 JSON game document.
// The DB is source of truth; this document is a complete lossless projection.
// Export reads from DB; import writes to DB; round-trips are lossless.
type GameDoc struct {
	Rapua     string       `json:"rapua"`
	ID        string       `json:"id,omitempty"`
	Name      string       `json:"name"`
	Settings  SettingsDoc  `json:"settings"`
	Start     []BlockDoc   `json:"start"`
	Finish    []BlockDoc   `json:"finish"`
	Structure StructureDoc `json:"structure"`
}

// SettingsDoc mirrors InstanceSettings fields.
type SettingsDoc struct {
	MustCheckOut      bool `json:"must_check_out"`
	ShowTeamCount     bool `json:"show_team_count"`
	EnablePoints      bool `json:"enable_points"`
	EnableBonusPoints bool `json:"enable_bonus_points"`
	ShowLeaderboard   bool `json:"show_leaderboard"`
}

// StructureDoc represents the root group of the game structure.
type StructureDoc struct {
	Routing         RouteStrategy  `json:"routing"`
	Navigation      NavigationMode `json:"navigation"`
	Completion      CompletionType `json:"completion"`
	MinimumRequired int            `json:"minimum_required,omitempty"`
	Children        []ChildDoc     `json:"children"`
}

// GroupDoc represents a named group within the structure tree.
type GroupDoc struct {
	ID              string         `json:"id,omitempty"`
	Name            string         `json:"name"`
	Color           string         `json:"color"`
	Routing         RouteStrategy  `json:"routing"`
	Navigation      NavigationMode `json:"navigation"`
	Completion      CompletionType `json:"completion"`
	MinimumRequired int            `json:"minimum_required,omitempty"`
	Children        []ChildDoc     `json:"children"`
}

// LocationDoc represents a single game location with its blocks.
type LocationDoc struct {
	ID         string     `json:"id,omitempty"`
	Slug       string     `json:"slug"`
	Name       string     `json:"name"`
	Points     int        `json:"points,omitempty"`
	Marker     *MarkerDoc `json:"marker,omitempty"`
	Content    []BlockDoc `json:"content"`
	Clues      []BlockDoc `json:"clues,omitempty"`
	Tasks      []BlockDoc `json:"tasks,omitempty"`
	Checkpoint []BlockDoc `json:"checkpoint,omitempty"`
}

// MarkerDoc holds geographic coordinates for a location.
type MarkerDoc struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// ChildDoc is a tagged union — exactly one of Location or Group is set.
// Custom marshal/unmarshal is in doc_marshal.go.
type ChildDoc struct {
	Location *LocationDoc
	Group    *GroupDoc
}

// BlockDoc is a flat map with a "type" discriminator plus all block-specific fields.
// The "id" and "points" keys are promoted alongside block-specific fields.
// On export, "id" is included. On create-import, "id" is omitted so new UUIDs are generated.
type BlockDoc map[string]any
