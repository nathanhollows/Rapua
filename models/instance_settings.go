package models

type InstanceSettings struct {
	baseModel

	InstanceID            string                `bun:"instance_id,pk,type:varchar(36)"`
	RouteStrategy         RouteStrategy         `bun:"navigation_mode,type:int"`
	NavigationDisplayMode NavigationDisplayMode `bun:"navigation_method,type:int"`
	MaxNextLocations      int                   `bun:"max_next_locations,type:int,default:3"`
	MustCheckOut          bool                  `bun:"must_check_out,type:bool"`
	ShowTeamCount         bool                  `bun:"show_team_count,type:bool"`
	EnablePoints          bool                  `bun:"enable_points,type:bool"`
	EnableBonusPoints     bool                  `bun:"enable_bonus_points,type:bool"`
	ShowLeaderboard       bool                  `bun:"show_leaderboard,type:bool"`
}
