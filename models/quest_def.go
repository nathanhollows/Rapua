package models

// QuestDef is the top-level YAML definition for a quest.
type QuestDef struct {
	Version   int          `yaml:"version"`
	ID        string       `yaml:"id,omitempty"`
	Name      string       `yaml:"name"`
	Settings  SettingsDef  `yaml:"settings,omitempty"`
	Structure StructureDef `yaml:"structure,omitempty"`
	Stops     []StopDef    `yaml:"stops,omitempty"`
	Start     []BlockDef   `yaml:"start,omitempty"`
	Finish    []BlockDef   `yaml:"finish,omitempty"`
}

// SettingsDef maps to InstanceSettings fields.
type SettingsDef struct {
	MustCheckOut     bool `yaml:"must_check_out,omitempty"`
	ShowTeamCount    bool `yaml:"show_team_count,omitempty"`
	EnablePoints     bool `yaml:"enable_points,omitempty"`
	EnableBonusPoints bool `yaml:"enable_bonus_points,omitempty"`
	ShowLeaderboard  bool `yaml:"show_leaderboard,omitempty"`
}

// StructureDef represents the game structure with stages.
type StructureDef struct {
	Stages []StageDef `yaml:"stages,omitempty"`
}

// StageDef represents a stage (group) in the game structure.
type StageDef struct {
	Name            string     `yaml:"name"`
	Color           string     `yaml:"color,omitempty"`
	Routing         string     `yaml:"routing,omitempty"`
	Navigation      string     `yaml:"navigation,omitempty"`
	Completion      string     `yaml:"completion,omitempty"`
	MinimumRequired int        `yaml:"minimum_required,omitempty"`
	MaxNext         int        `yaml:"max_next,omitempty"`
	AutoAdvance     bool       `yaml:"auto_advance,omitempty"`
	Stops           []string   `yaml:"stops,omitempty"`
	Stages          []StageDef `yaml:"stages,omitempty"`
}

// StopDef represents a location/stop in the quest.
type StopDef struct {
	ID         string     `yaml:"id,omitempty"`
	Slug       string     `yaml:"slug"`
	Name       string     `yaml:"name"`
	Points     int        `yaml:"points,omitempty"`
	Marker     *MarkerDef `yaml:"marker,omitempty"`
	Content    []BlockDef `yaml:"content,omitempty"`
	Clues      []BlockDef `yaml:"clues,omitempty"`
	Tasks      []BlockDef `yaml:"tasks,omitempty"`
	Checkpoint []BlockDef `yaml:"checkpoint,omitempty"`
}

// MarkerDef represents a map marker with coordinates.
type MarkerDef struct {
	Lat float64 `yaml:"lat"`
	Lng float64 `yaml:"lng"`
}
