package models

type InstanceSettings struct {
	baseModel

	InstanceID        string           `bun:"instance_id,pk,type:varchar(36)"`
	NavigationMode    NavigationMode   `bun:"navigation_mode,type:int"`
	NavigationMethod  NavigationMethod `bun:"navigation_method,type:int"`
	MaxNextLocations  int              `bun:"max_next_locations,type:int,default:3"`
	CompletionMethod  CompletionMethod `bun:"completion_method,type:int"`
	ShowTeamCount     bool             `bun:"show_team_count,type:bool"`
	EnablePoints      bool             `bun:"enable_points,type:bool"`
	EnableBonusPoints bool             `bun:"enable_bonus_points,type:bool"`
	ShowLeaderboard   bool             `bun:"show_leaderboard,type:bool"`
}
